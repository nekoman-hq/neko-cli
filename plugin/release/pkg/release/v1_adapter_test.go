//nolint:staticcheck // These tests protect the deprecated V1 compatibility boundary.
package release

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type recordingV1GitRunner struct {
	commands []string
	roots    []string
}

func (runner *recordingV1GitRunner) CombinedOutput(root string, args ...string) ([]byte, error) {
	runner.roots = append(runner.roots, root)
	runner.commands = append(runner.commands, strings.Join(args, " "))
	if len(args) > 0 && args[0] == "rev-parse" {
		return []byte("abc123\n"), nil
	}
	return nil, nil
}

func TestSystemV1GitWriterUsesExactLegacyCommandsInRepository(t *testing.T) {
	runner := &recordingV1GitRunner{}
	writer := &SystemV1GitWriter{runner: runner}
	version := semver.MustParse("1.2.4")

	if head, err := writer.Head("/repo"); err != nil || head != "abc123" {
		t.Fatalf("Head = %q, %v", head, err)
	}
	if err := writer.CreateReleaseCommit("/repo", version); err != nil {
		t.Fatalf("CreateReleaseCommit: %v", err)
	}
	if err := writer.CreateGitTag("/repo", version); err != nil {
		t.Fatalf("CreateGitTag: %v", err)
	}
	if err := writer.PushCommits("/repo"); err != nil {
		t.Fatalf("PushCommits: %v", err)
	}
	if err := writer.PushGitTag("/repo", version); err != nil {
		t.Fatalf("PushGitTag: %v", err)
	}
	want := []string{
		"rev-parse --short HEAD",
		"commit --allow-empty -a -m chore(neko-release): 1.2.4",
		"tag v1.2.4",
		"push origin HEAD",
		"push origin v1.2.4",
	}
	if strings.Join(runner.commands, "\n") != strings.Join(want, "\n") {
		t.Fatalf("commands = %v, want %v", runner.commands, want)
	}
	for _, root := range runner.roots {
		if root != "/repo" {
			t.Fatalf("command root = %q, want /repo", root)
		}
	}
}

type recordingV1RollbackGit struct {
	events []string
}

func (git *recordingV1RollbackGit) DeleteLocalTag(_, tag string) error {
	git.events = append(git.events, "local-tag "+tag)
	return nil
}
func (git *recordingV1RollbackGit) DeleteRemoteTag(_, tag string) error {
	git.events = append(git.events, "remote-tag "+tag)
	return nil
}
func (git *recordingV1RollbackGit) RevertCommit(_, hash string) error {
	git.events = append(git.events, "revert "+hash)
	return nil
}
func (git *recordingV1RollbackGit) CreateFallbackCommit(_, message string) error {
	git.events = append(git.events, "fallback "+message)
	return nil
}
func (git *recordingV1RollbackGit) PushCommits(string) error {
	git.events = append(git.events, "push")
	return nil
}
func (git *recordingV1RollbackGit) HardResetTo(_, hash string) error {
	git.events = append(git.events, "reset "+hash)
	return nil
}
func (git *recordingV1RollbackGit) CleanUntracked(string) error {
	git.events = append(git.events, "clean")
	return nil
}

type recordingV1ReleaseRemover struct {
	events *[]string
}

func (remover recordingV1ReleaseRemover) Delete(_, tag string) error {
	*remover.events = append(*remover.events, "github "+tag)
	return nil
}

func TestV1ReleaseRollbackKeepsFocusedCompensationOrder(t *testing.T) {
	git := &recordingV1RollbackGit{}
	rollback := &V1ReleaseRollback{git: git, releases: recordingV1ReleaseRemover{events: &git.events}}
	err := rollback.Rollback("/repo", GitReleaseState{
		ReleaseHead:          "release",
		TagName:              "v1.2.4",
		GitHubReleaseTag:     "v1.2.4",
		PushedCommit:         true,
		PushedTag:            true,
		CreatedGitHubRelease: true,
	})
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	want := []string{"github v1.2.4", "local-tag v1.2.4", "remote-tag v1.2.4", "revert release", "push", "clean"}
	if strings.Join(git.events, "\n") != strings.Join(want, "\n") {
		t.Fatalf("events = %v, want %v", git.events, want)
	}
}

type fixedV1TokenResolver struct{ token string }

func (resolver fixedV1TokenResolver) Resolve() (v1GitHubToken, error) {
	return v1GitHubToken{value: resolver.token}, nil
}

type failingV1ReleaseClient struct{ cause error }

func (client failingV1ReleaseClient) Delete(_, _ string, _ v1GitHubToken) error { return client.cause }

func TestV1GitHubReleaseRemoverRedactsTokenAndPreservesCause(t *testing.T) {
	secret := "v1-sentinel-secret"
	cause := errors.New("remote echoed " + secret)
	remover := systemV1GitHubReleaseRemover{
		tokens: fixedV1TokenResolver{token: secret},
		client: failingV1ReleaseClient{cause: cause},
	}
	err := remover.Delete("/repo", "v1.2.4")
	if err == nil || strings.Contains(err.Error(), secret) || !errors.Is(err, cause) {
		t.Fatalf("redacted error = %v, preserves cause=%t", err, errors.Is(err, cause))
	}
}

func TestV1ProcessResultRedactionIsSharedWithoutLosingCause(t *testing.T) {
	secret := "executor-sentinel-secret"
	cause := errors.New("process exposed " + secret)
	output, err := RedactV1ProcessResultFromEnvironment(
		[]byte("output "+secret),
		cause,
		[]string{"VISIBLE=yes", "GITHUB_TOKEN=" + secret},
	)
	if strings.Contains(string(output), secret) || strings.Contains(err.Error(), secret) || !errors.Is(err, cause) {
		t.Fatalf("output=%q error=%v preserves cause=%t", output, err, errors.Is(err, cause))
	}
}

type recordingV1ConfigStore struct {
	roots    []string
	versions []string
}

func (store *recordingV1ConfigStore) Save(root string, cfg releaseconfig.V1ReleaseConfig) error {
	store.roots = append(store.roots, root)
	store.versions = append(store.versions, cfg.Version)
	return nil
}

func TestV1ConfigMaterializerOwnsCanonicalWriteAndRestore(t *testing.T) {
	store := &recordingV1ConfigStore{}
	materializer := v1ReleaseConfigFileMaterializer{store: store}
	intent := V1ReleaseIntent{RepositoryRoot: "/repo", Config: &releaseconfig.V1ReleaseConfig{Version: "1.2.3"}}
	plan := V1ReleasePlan{CurrentVersion: "1.2.3", NextVersion: "1.2.4"}
	if err := materializer.WritePlannedVersion(intent, plan); err != nil {
		t.Fatalf("WritePlannedVersion: %v", err)
	}
	if err := materializer.RestorePreviousVersion(intent, plan); err != nil {
		t.Fatalf("RestorePreviousVersion: %v", err)
	}
	if got := fmt.Sprint(store.roots, store.versions); got != "[/repo /repo] [1.2.4 1.2.3]" {
		t.Fatalf("materialization evidence = %s", got)
	}
}

func TestFixedV1ExecutorCatalogDoesNotReadCompatibilityRegistry(t *testing.T) {
	events := []string{}
	executor := &recordingV1Executor{events: &events}
	catalog := newFixedV1ReleaseExecutorCatalog(executor)
	original := tools
	tools = map[string]Tool{}
	t.Cleanup(func() { tools = original })

	resolved, err := catalog.Resolve("goreleaser")
	if err != nil || resolved != executor {
		t.Fatalf("Resolve = %T, %v", resolved, err)
	}
	if _, err := catalog.Resolve("missing"); err == nil || err.Error() != "unknown release system: missing" {
		t.Fatalf("missing executor error = %v", err)
	}
}

type recordingV1TokenResolver struct{ calls int }

func (resolver *recordingV1TokenResolver) Resolve() (v1GitHubToken, error) {
	resolver.calls++
	return v1GitHubToken{value: "sentinel"}, nil
}

type selectiveV1FileInspector struct {
	exists string
	paths  []string
}

func (inspector *selectiveV1FileInspector) Exists(root, path string) (bool, error) {
	path = filepath.Join(root, path)
	inspector.paths = append(inspector.paths, path)
	return path == inspector.exists, nil
}

func TestV1RequirementsResolveLegacyTokenAndExecutorFilesOnce(t *testing.T) {
	tokens := &recordingV1TokenResolver{}
	files := &selectiveV1FileInspector{exists: "/repo/.goreleaser.yaml"}
	requirements := systemV1ReleaseRequirements{tokens: tokens, files: files}
	err := requirements.Validate(V1ReleaseIntent{
		RepositoryRoot: "/repo",
		Config:         &releaseconfig.V1ReleaseConfig{ReleaseSystem: releaseconfig.V1ReleaseTypeGoReleaser},
	})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if tokens.calls != 1 {
		t.Fatalf("token resolutions = %d, want 1", tokens.calls)
	}
	want := []string{"/repo/.goreleaser.yml", "/repo/.goreleaser.yaml"}
	if strings.Join(files.paths, "\n") != strings.Join(want, "\n") {
		t.Fatalf("file checks = %v, want %v", files.paths, want)
	}
}

type successfulV1Requirements struct{ calls int }

func (requirements *successfulV1Requirements) Validate(V1ReleaseIntent) error {
	requirements.calls++
	return nil
}

type recordingV1PreflightRepository struct {
	failAt string
	events []string
}

func (repository *recordingV1PreflightRepository) record(event string) error {
	repository.events = append(repository.events, event)
	if event == repository.failAt {
		return errors.New("preflight failed")
	}
	return nil
}
func (repository *recordingV1PreflightRepository) Observe(string) error {
	return repository.record("observe")
}
func (repository *recordingV1PreflightRepository) IsClean(string) error {
	return repository.record("clean")
}
func (repository *recordingV1PreflightRepository) EnsureNotDetached(string) error {
	return repository.record("attached")
}
func (repository *recordingV1PreflightRepository) OnMainBranch(string) error {
	return repository.record("main")
}
func (repository *recordingV1PreflightRepository) HasUpstream(string) error {
	return repository.record("upstream")
}
func (repository *recordingV1PreflightRepository) IsUpToDate(string) error {
	return repository.record("current")
}

func TestV1PreflightStopsAtFirstRepositoryFailure(t *testing.T) {
	requirements := &successfulV1Requirements{}
	repository := &recordingV1PreflightRepository{failAt: "main"}
	failure := (legacyV1Preflight{requirements: requirements, repository: repository}).Check(V1ReleaseIntent{})
	if failure == nil || failure.Code != "INCORRECT_BRANCH" || failure.Cause.Error() != "preflight failed" {
		t.Fatalf("failure = %#v", failure)
	}
	want := []string{"clean", "attached", "main"}
	if strings.Join(repository.events, "\n") != strings.Join(want, "\n") {
		t.Fatalf("events = %v, want %v", repository.events, want)
	}
	if requirements.calls != 1 {
		t.Fatalf("requirement calls = %d, want 1", requirements.calls)
	}
}
