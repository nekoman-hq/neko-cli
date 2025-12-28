// Package goreleaser includes the goreleaser release-system logic
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

func (g *GoReleaser) Init(_ *config.NekoConfig) error {
	g.RequireBinary(g.Name())

	runGoreleaserInit()
	runGoreleaserCheck()
	return nil
}

func (g *GoReleaser) SupportsSurvey() bool {
	return true
}

func (g *GoReleaser) Release(v *semver.Version) error {
	if err := g.CreateReleaseCommit(v); err != nil {
		return err
	}

	if err := g.CreateGitTag(v); err != nil {
		return err
	}

	if err := g.PushCommits(); err != nil {
		return err
	}

	if err := g.PushGitTag(v); err != nil {
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
			"Skipping goreleaser init, %s already exists",
			log.ColorText(log.ColorCyan, "goreleaser.yml"),
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
		"\uF00C  Successfully initialized %s",
		log.ColorText(log.ColorCyan, "goreleaser"),
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
		"\uF00C Configuration check passed for %s",
		log.ColorText(log.ColorCyan, "goreleaser"),
	)
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
		log.Print(log.Release, "\u26A0 Dry run failed, but continuing with release")
		return nil
	}

	log.Print(log.Release, "\uF00C GoReleaser dry run %s",
		log.ColorText(log.ColorGreen, "successful"))
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

	log.Print(log.Release, "\uF00C GoReleaser release %s",
		log.ColorText(log.ColorGreen, "successful"),
	)
	return nil
}

func (g *GoReleaser) Survey(v *semver.Version) (release.Type, error) {
	return release.NekoSurvey(v)
}

func init() {
	release.Register(&GoReleaser{})
}
