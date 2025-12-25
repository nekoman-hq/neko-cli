package goreleaser

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/log"
	"github.com/nekoman-hq/neko-cli/internal/release"
)

type GoReleaser struct {
	release.ToolBase
}

func (g *GoReleaser) Name() string {
	return "goreleaser"
}

func (g *GoReleaser) Init(_ *semver.Version, _ *config.NekoConfig) error {

	g.RequireBinary(g.Name())

	runGoreleaserInit()
	runGoreleaserCheck()
	return nil
}

func (g *GoReleaser) SupportsSurvey() bool {
	return true
}

func (g *GoReleaser) Release(v *semver.Version) error {

	if err := g.createReleaseCommit(v); err != nil {
		return err
	}

	if err := g.createGitTag(v); err != nil {
		return err
	}

	if err := g.pushCommits(); err != nil {
		return err
	}

	if err := g.pushGitTag(v); err != nil {
		return err
	}

	if err := g.runGoReleaserDryRun(); err != nil {
		return err
	}

	if err := g.runGoReleaserRelease(); err != nil {
		return err
	}

	return nil
}

func runGoreleaserInit() {

	if _, err := os.Stat(".goreleaser.yaml"); err == nil {
		log.Print(
			log.Init,
			fmt.Sprintf(
				"Skipping goreleaser init, %s already exists",
				log.ColorText(log.ColorCyan, "goreleaser.yml"),
			),
		)
		return
	} else if !os.IsNotExist(err) {
		errors.Fatal(
			"Failed to check goreleaser.yml",
			err.Error(),
			errors.ErrFileAccess,
		)
		return
	}

	log.V(log.Init,
		fmt.Sprintf("Initializing goreleaser: %s",
			log.ColorText(log.ColorGreen, "goreleaser init"),
		),
	)

	cmd := exec.Command("goreleaser", "init")
	output, err := cmd.CombinedOutput()

	if err != nil {
		errors.Fatal(
			"Failed to initialize goreleaser",
			fmt.Sprintf("Command failed: %s\nOutput: %s", err.Error(), string(output)),
			errors.ErrDependencyMissing,
		)
	}

	log.Print(
		log.Init,
		fmt.Sprintf(
			"\uF00C  Successfully initialized %s",
			log.ColorText(log.ColorCyan, "goreleaser"),
		),
	)
}

func runGoreleaserCheck() {
	log.V(log.Init,
		fmt.Sprintf("Checking goreleaser configuration: %s",
			log.ColorText(log.ColorGreen, "goreleaser check"),
		),
	)

	cmd := exec.Command("goreleaser", "check")
	output, err := cmd.CombinedOutput()

	if err != nil {
		errors.Fatal(
			"Goreleaser configuration check failed",
			fmt.Sprintf("Command failed: %s\nOutput: %s", err.Error(), string(output)),
			errors.ErrDependencyMissing,
		)
	}

	log.Print(
		log.Init,
		fmt.Sprintf(
			"\uF00C Configuration check passed for %s",
			log.ColorText(log.ColorCyan, "goreleaser"),
		),
	)
}

// createReleaseCommit creates the chore commit for the release
func (g *GoReleaser) createReleaseCommit(v *semver.Version) error {
	commitMsg := fmt.Sprintf("chore(neko-release): %s", v)

	log.V(log.Release, fmt.Sprintf("Creating release commit: %s",
		log.ColorText(log.ColorGreen, fmt.Sprintf("git commit --allow-empty -m \"%s\"", commitMsg))))

	cmd := exec.Command("git", "commit", "--allow-empty", "-a", "-m", commitMsg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errors.Fatal(
			"Failed to create release commit",
			fmt.Sprintf("git commit failed: %s", strings.TrimSpace(string(output))),
			errors.ErrReleaseCommit,
		)
	}

	log.Print(log.Release, fmt.Sprintf("\uF00C Created release commit: %s",
		log.ColorText(log.ColorGreen, commitMsg)))
	return nil
}

// createGitTag creates a git tag for the version
func (g *GoReleaser) createGitTag(v *semver.Version) error {
	tag := fmt.Sprintf("v%s", v)

	log.V(log.Release, fmt.Sprintf("Creating git tag: %s",
		log.ColorText(log.ColorGreen, fmt.Sprintf("git tag %s", tag))))

	cmd := exec.Command("git", "tag", tag)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errors.Fatal(
			"Failed to create git tag",
			fmt.Sprintf("git tag %s failed: %s", tag, strings.TrimSpace(string(output))),
			errors.ErrReleaseTag,
		)
	}

	log.Print(log.Release, fmt.Sprintf("\uF00C Created git tag: %s",
		log.ColorText(log.ColorGreen, tag)))
	return nil
}

// pushCommit pushes the release commit to remote
func (g *GoReleaser) pushCommits() error {
	log.V(log.Release, fmt.Sprintf("Pushing release commit: %s",
		log.ColorText(log.ColorGreen, "git push origin HEAD")))

	cmd := exec.Command("git", "push", "origin", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		errors.Fatal(
			"Failed to push release commits",
			fmt.Sprintf("git push failed: %s", strings.TrimSpace(string(output))),
			errors.ErrReleasePush,
		)
	}

	log.Print(log.Release, fmt.Sprintf("\uF00C Pushed release commit to %s",
		log.ColorText(log.ColorGreen, "origin")))
	return nil
}

// pushGitTag pushes the git tag to remote
func (g *GoReleaser) pushGitTag(v *semver.Version) error {
	tag := fmt.Sprintf("v%s", v)

	log.V(log.Release, fmt.Sprintf("Pushing git tag: %s",
		log.ColorText(log.ColorGreen, fmt.Sprintf("git push origin %s", tag))))

	cmd := exec.Command("git", "push", "origin", tag)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errors.Fatal(
			"Failed to push git tag",
			fmt.Sprintf("git push %s failed: %s", tag, strings.TrimSpace(string(output))),
			errors.ErrReleasePush,
		)
	}

	log.Print(log.Release, fmt.Sprintf("\uF00C Pushed git tag: %s",
		log.ColorText(log.ColorGreen, tag)))
	return nil
}

// runGoReleaserDryRun executes goreleaser in dry-run mode
func (g *GoReleaser) runGoReleaserDryRun() error {
	log.V(log.Release, fmt.Sprintf("Running GoReleaser dry run: %s",
		log.ColorText(log.ColorGreen, "goreleaser release --snapshot --clean")))

	cmd := exec.Command("goreleaser", "release", "--snapshot", "--clean")
	output, err := cmd.CombinedOutput()
	if err != nil {
		errors.Warning(
			"GoReleaser dry run failed",
			fmt.Sprintf("This is a warning - proceeding anyway: %s", strings.TrimSpace(string(output))),
		)
		log.Print(log.Release, fmt.Sprintf("\u26A0 Dry run failed, but continuing with release"))
		return nil
	}

	log.Print(log.Release, fmt.Sprintf("\uF00C GoReleaser dry run %s",
		log.ColorText(log.ColorGreen, "successful")))
	return nil
}

// runGoReleaserRelease executes the full goreleaser release
func (g *GoReleaser) runGoReleaserRelease() error {
	log.V(log.Release, fmt.Sprintf("Running GoReleaser release: %s",
		log.ColorText(log.ColorGreen, "goreleaser release --clean")))

	cmd := exec.Command("goreleaser", "release", "--clean")
	output, err := cmd.CombinedOutput()
	if err != nil {
		errors.Fatal(
			"GoReleaser release failed",
			fmt.Sprintf("goreleaser release failed: %s", strings.TrimSpace(string(output))),
			errors.ErrGoReleaserExecution,
		)
	}

	log.Print(log.Release, fmt.Sprintf("\uF00C GoReleaser release %s",
		log.ColorText(log.ColorGreen, "successful")))
	return nil
}

func (g *GoReleaser) Survey(version *semver.Version) (release.Type, error) {
	return release.NekoSurvey(version)
}

func init() {
	release.Register(&GoReleaser{})
}
