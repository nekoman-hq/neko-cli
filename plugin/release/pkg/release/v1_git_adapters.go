//nolint:staticcheck // V1 compatibility adapters intentionally use the deprecated token contract.
package release

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver/v3"
	coreconfig "github.com/nekoman-hq/neko-cli/pkg/config"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	legacygit "github.com/nekoman-hq/neko-cli/plugin/release/pkg/git"
)

type v1GitCommandRunner interface {
	CombinedOutput(repositoryRoot string, args ...string) ([]byte, error)
}

type v1GitEnvironment interface {
	Environ() []string
}

type systemV1GitEnvironment struct{}

func (systemV1GitEnvironment) Environ() []string { return os.Environ() }

type systemV1GitCommandRunner struct {
	environment v1GitEnvironment
}

func newSystemV1GitCommandRunner() systemV1GitCommandRunner {
	return systemV1GitCommandRunner{environment: systemV1GitEnvironment{}}
}

func (runner systemV1GitCommandRunner) CombinedOutput(repositoryRoot string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repositoryRoot
	environment := runner.environment.Environ()
	cmd.Env = environment
	output, err := cmd.CombinedOutput()
	return RedactV1ProcessResultFromEnvironment(output, err, environment)
}

// SystemV1GitWriter owns the exact legacy commit, tag, and push commands. It is
// deliberately separate from V2 Git coordination because V1 uses commit -a,
// an allow-empty commit, a fixed v tag prefix, and immediate pushes.
type SystemV1GitWriter struct {
	runner v1GitCommandRunner
}

func NewSystemV1GitWriter() *SystemV1GitWriter {
	return &SystemV1GitWriter{runner: newSystemV1GitCommandRunner()}
}

func (writer *SystemV1GitWriter) Head(repositoryRoot string) (string, error) {
	output, err := writer.runner.CombinedOutput(repositoryRoot, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (writer *SystemV1GitWriter) CreateReleaseCommit(repositoryRoot string, version *semver.Version) error {
	message := fmt.Sprintf("chore(neko-release): %s", version)
	log.PluginV(log.Exec, fmt.Sprintf("Creating release commit: %s",
		log.ColorText(log.ColorGreen, fmt.Sprintf("git commit --allow-empty -m \"%s\"", message))))
	output, err := writer.runner.CombinedOutput(repositoryRoot, "commit", "--allow-empty", "-a", "-m", message)
	if err != nil {
		return fmt.Errorf("failed to create release commit: %s: %w", string(output), err)
	}
	log.PluginPrint(log.Exec, "\uF00C Created release commit: %s", log.ColorText(log.ColorGreen, message))
	return nil
}

func (writer *SystemV1GitWriter) CreateGitTag(repositoryRoot string, version *semver.Version) error {
	tag := fmt.Sprintf("v%s", version)
	log.PluginV(log.Exec, fmt.Sprintf("Creating git tag: %s",
		log.ColorText(log.ColorGreen, fmt.Sprintf("git tag %s", tag))))
	output, err := writer.runner.CombinedOutput(repositoryRoot, "tag", tag)
	if err != nil {
		return fmt.Errorf("failed to create git tag: %s: %w", string(output), err)
	}
	log.PluginPrint(log.Exec, "\uF00C Created git tag: %s", log.ColorText(log.ColorGreen, tag))
	return nil
}

func (writer *SystemV1GitWriter) PushCommits(repositoryRoot string) error {
	log.PluginV(log.Exec, fmt.Sprintf("Pushing release commit: %s",
		log.ColorText(log.ColorGreen, "git push origin HEAD")))
	output, err := writer.runner.CombinedOutput(repositoryRoot, "push", "origin", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to push release commits: %s: %w", string(output), err)
	}
	log.PluginPrint(log.Exec, "\uF00C Pushed release commit to %s", log.ColorText(log.ColorGreen, "origin"))
	return nil
}

func (writer *SystemV1GitWriter) PushGitTag(repositoryRoot string, version *semver.Version) error {
	tag := fmt.Sprintf("v%s", version)
	log.PluginV(log.Exec, fmt.Sprintf("Pushing git tag: %s",
		log.ColorText(log.ColorGreen, fmt.Sprintf("git push origin %s", tag))))
	output, err := writer.runner.CombinedOutput(repositoryRoot, "push", "origin", tag)
	if err != nil {
		return fmt.Errorf("failed to push git tag: %s: %w", string(output), err)
	}
	log.PluginPrint(log.Exec, "\uF00C Pushed git tag: %s", log.ColorText(log.ColorGreen, tag))
	return nil
}

type v1RollbackGit interface {
	DeleteLocalTag(string, string) error
	DeleteRemoteTag(string, string) error
	RevertCommit(string, string) error
	CreateFallbackCommit(string, string) error
	PushCommits(string) error
	HardResetTo(string, string) error
	CleanUntracked(string) error
}

type v1GitHubReleaseRemover interface {
	Delete(string, string) error
}

// V1ReleaseRollback owns only the bounded legacy compensation sequence. It
// does not model phases or promise transactional recovery.
type V1ReleaseRollback struct {
	git      v1RollbackGit
	releases v1GitHubReleaseRemover
}

func NewSystemV1ReleaseRollback() *V1ReleaseRollback {
	runner := newSystemV1GitCommandRunner()
	return &V1ReleaseRollback{
		git:      systemV1RollbackGit{runner: runner},
		releases: newSystemV1GitHubReleaseRemover(),
	}
}

func (rollback *V1ReleaseRollback) Rollback(repositoryRoot string, state GitReleaseState) error {
	if !state.hasMutatingStep() {
		log.PluginV(log.Guard, "Skipping rollback because no mutating release step was recorded")
		return nil
	}
	if state.CreatedGitHubRelease && state.GitHubReleaseTag != "" {
		if err := rollback.releases.Delete(repositoryRoot, state.GitHubReleaseTag); err != nil {
			return fmt.Errorf("rollback: failed deleting GitHub release %s: %w", state.GitHubReleaseTag, err)
		}
	}
	if state.TagName != "" {
		_ = rollback.git.DeleteLocalTag(repositoryRoot, state.TagName)
		if state.PushedTag {
			if err := rollback.git.DeleteRemoteTag(repositoryRoot, state.TagName); err != nil {
				return fmt.Errorf("rollback: failed deleting remote tag %s: %w", state.TagName, err)
			}
		}
	}
	if state.ReleaseHead != "" {
		if state.PushedCommit {
			if err := rollback.git.RevertCommit(repositoryRoot, state.ReleaseHead); err != nil {
				_ = rollback.git.CreateFallbackCommit(repositoryRoot, fmt.Sprintf("revert %s", state.ReleaseHead))
			}
			if err := rollback.git.PushCommits(repositoryRoot); err != nil {
				return fmt.Errorf("rollback: failed pushing revert commit: %w", err)
			}
		} else if state.PreHead != "" {
			if err := rollback.git.HardResetTo(repositoryRoot, state.PreHead); err != nil {
				return fmt.Errorf("rollback: failed hard reset to %s: %w", state.PreHead, err)
			}
		} else {
			return fmt.Errorf("rollback: inconsistent state (release commit exists but pre-head missing)")
		}
	}
	if err := rollback.git.CleanUntracked(repositoryRoot); err != nil {
		return fmt.Errorf("rollback: failed cleaning untracked files: %w", err)
	}
	return nil
}

type systemV1RollbackGit struct {
	runner v1GitCommandRunner
}

func (adapter systemV1RollbackGit) run(root string, description string, args ...string) error {
	output, err := adapter.runner.CombinedOutput(root, args...)
	if err != nil {
		return fmt.Errorf("%s: %s: %w", description, strings.TrimSpace(string(output)), err)
	}
	return nil
}

func (adapter systemV1RollbackGit) DeleteLocalTag(root, tag string) error {
	return adapter.run(root, fmt.Sprintf("git tag -d %s failed", tag), "tag", "-d", tag)
}
func (adapter systemV1RollbackGit) DeleteRemoteTag(root, tag string) error {
	return adapter.run(root, fmt.Sprintf("git push origin --delete %s failed", tag), "push", "origin", "--delete", tag)
}
func (adapter systemV1RollbackGit) RevertCommit(root, hash string) error {
	return adapter.run(root, fmt.Sprintf("git revert %s failed", hash), "revert", "--no-edit", hash)
}
func (adapter systemV1RollbackGit) CreateFallbackCommit(root, message string) error {
	return adapter.run(root, fmt.Sprintf("git commit -m %q failed", message), "commit", "--allow-empty", "-m", message)
}
func (adapter systemV1RollbackGit) PushCommits(root string) error {
	return adapter.run(root, "git push origin HEAD failed", "push", "origin", "HEAD")
}
func (adapter systemV1RollbackGit) HardResetTo(root, hash string) error {
	return adapter.run(root, fmt.Sprintf("git reset --hard %s failed", hash), "reset", "--hard", hash)
}
func (adapter systemV1RollbackGit) CleanUntracked(root string) error {
	return adapter.run(root, "git clean -fd failed", "clean", "-fd")
}

type v1LegacyTokenResolver interface {
	Resolve() (string, error)
}

type systemV1LegacyTokenResolver struct{}

func (systemV1LegacyTokenResolver) Resolve() (string, error) { return coreconfig.GetPAT() }

type v1GitHubReleaseClient interface {
	Delete(repositoryRoot, tag, token string) error
}

type legacyV1GitHubReleaseClient struct{}

func (legacyV1GitHubReleaseClient) Delete(repositoryRoot, tag, token string) error {
	return inV1Repository(repositoryRoot, func() error {
		return legacygit.DeleteGithubRelease(tag, token)
	})
}

func inV1Repository(repositoryRoot string, operation func() error) error {
	if repositoryRoot == "" {
		return operation()
	}
	previous, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to resolve current working directory: %w", err)
	}
	if err := os.Chdir(repositoryRoot); err != nil {
		return fmt.Errorf("failed to enter repository root %s: %w", repositoryRoot, err)
	}
	defer func() { _ = os.Chdir(previous) }()
	return operation()
}

type systemV1GitHubReleaseRemover struct {
	tokens v1LegacyTokenResolver
	client v1GitHubReleaseClient
}

func newSystemV1GitHubReleaseRemover() systemV1GitHubReleaseRemover {
	return systemV1GitHubReleaseRemover{
		tokens: systemV1LegacyTokenResolver{},
		client: legacyV1GitHubReleaseClient{},
	}
}

func (remover systemV1GitHubReleaseRemover) Delete(repositoryRoot, tag string) error {
	token, err := remover.tokens.Resolve()
	if err != nil {
		return err
	}
	if err := remover.client.Delete(repositoryRoot, tag, token); err != nil {
		_, redactedErr := RedactV1ProcessResult(nil, err, token)
		return redactedErr
	}
	return nil
}
