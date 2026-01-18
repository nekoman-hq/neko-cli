package release

import (
	"fmt"
	"os/exec"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/git"
	"github.com/nekoman-hq/neko-cli/internal/log"
)

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

type Tool interface {
	Name() string
	Init(cfg *config.NekoConfig) error
	Release(v *semver.Version) error
	Survey(v *semver.Version) (Type, error)
	SupportsSurvey() bool
	RevertRelease() error
}

type ToolBase struct{}

func (tb *ToolBase) RequireBinary(name string) error {
	log.V(log.Init,
		fmt.Sprintf("Searching for %s executable: %s",
			name,
			log.ColorText(log.ColorGreen, fmt.Sprintf("which %s", name)),
		),
	)

	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf(
			"Required dependency missing: %s: %w", path, err,
		)
	}

	log.Print(
		log.Init,
		"\uF00C Found %s at %s",
		log.ColorText(log.ColorCyan, name),
		log.ColorText(log.ColorGreen, path),
	)

	return nil
}

type GitReleaseState struct {
	PreHead     string
	ReleaseHead string

	TagName      string
	PushedCommit bool
	PushedTag    bool

	GitHubReleaseTag     string // usually same as TagName
	CreatedGitHubRelease bool
}

func (t *ToolBase) RevertGitRelease(st GitReleaseState) error {
	// GitHub release has to be deleted before the corresponding tag
	if st.CreatedGitHubRelease && st.GitHubReleaseTag != "" {
		if err := t.DeleteGitHubRelease(st.GitHubReleaseTag); err != nil {
			return fmt.Errorf(
				"rollback: failed deleting GitHub release %s: %w",
				st.GitHubReleaseTag,
				err,
			)
		}
	}

	// Tags
	if st.TagName != "" {
		_ = git.DeleteLocalTag(st.TagName)

		if st.PushedTag {
			if err := git.DeleteRemoteTag(st.TagName); err != nil {
				return fmt.Errorf(
					"rollback: failed deleting remote tag %s: %w",
					st.TagName,
					err,
				)
			}
		}
	}

	// Commits
	if st.ReleaseHead != "" {
		if st.PushedCommit {
			// empty commits cannot be reverted, ignore error
			if err := git.RevertCommit(st.ReleaseHead); err != nil {
				_ = git.CreateCommit(fmt.Sprintf("revert %s", st.ReleaseHead))
			}

			if err := t.PushCommits(); err != nil {
				return fmt.Errorf(
					"rollback: failed pushing revert commit: %w",
					err,
				)
			}
		} else if st.PreHead != "" {
			if err := git.HardResetTo(st.PreHead); err != nil {
				return fmt.Errorf(
					"rollback: failed hard reset to %s: %w",
					st.PreHead,
					err,
				)
			}
		} else {
			return fmt.Errorf(
				"rollback: inconsistent state (release commit exists but pre-head missing)",
			)
		}
	}

	// Final cleanup
	if err := git.CleanUntracked(); err != nil {
		return fmt.Errorf(
			"rollback: failed cleaning untracked files: %w",
			err,
		)
	}

	return nil
}

func (t *ToolBase) DeleteGitHubRelease(tag string) error {
	pat, err := config.GetPAT()
	if err != nil {
		return err
	}

	return git.DeleteGithubRelease(tag, pat)
}

// CreateReleaseCommit creates the chore commit for the release
func (tb *ToolBase) CreateReleaseCommit(v *semver.Version) error {
	commitMsg := fmt.Sprintf("chore(neko-release): %s", v)

	log.V(log.Release, fmt.Sprintf("Creating release commit: %s",
		log.ColorText(log.ColorGreen, fmt.Sprintf("git commit --allow-empty -m \"%s\"", commitMsg))))

	cmd := exec.Command("git", "commit", "--allow-empty", "-a", "-m", commitMsg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"Failed to create release commit: %s: %w", string(output), err,
		)
	}

	log.Print(log.Release, "\uF00C Created release commit: %s",
		log.ColorText(log.ColorGreen, commitMsg))
	return nil
}

// CreateGitTag creates a git tag for the version
func (tb *ToolBase) CreateGitTag(v *semver.Version) error {
	tag := fmt.Sprintf("v%s", v)

	log.V(log.Release, fmt.Sprintf("Creating git tag: %s",
		log.ColorText(log.ColorGreen, fmt.Sprintf("git tag %s", tag))))

	cmd := exec.Command("git", "tag", tag)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"Failed to create git tag: %s: %w", string(output), err,
		)
	}

	log.Print(log.Release, "\uF00C Created git tag: %s",
		log.ColorText(log.ColorGreen, tag))
	return nil
}

// PushCommits pushes the release commit to remote
func (tb *ToolBase) PushCommits() error {
	log.V(log.Release, fmt.Sprintf("Pushing release commit: %s",
		log.ColorText(log.ColorGreen, "git push origin HEAD")))

	cmd := exec.Command("git", "push", "origin", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"Failed to push release commits: %s: %w", string(output), err,
		)
	}

	log.Print(log.Release, "\uF00C Pushed release commit to %s",
		log.ColorText(log.ColorGreen, "origin"))
	return nil
}

// PushGitTag pushes the git tag to remote
func (tb *ToolBase) PushGitTag(v *semver.Version) error {
	tag := fmt.Sprintf("v%s", v)

	log.V(log.Release, fmt.Sprintf("Pushing git tag: %s",
		log.ColorText(log.ColorGreen, fmt.Sprintf("git push origin %s", tag))))

	cmd := exec.Command("git", "push", "origin", tag)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"Failed to push git tag: %s: %w", string(output), err,
		)
	}

	log.Print(log.Release, "\uF00C Pushed git tag: %s",
		log.ColorText(log.ColorGreen, tag))
	return nil
}
