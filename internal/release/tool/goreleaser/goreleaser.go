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
	"github.com/nekoman-hq/neko-cli/internal/git"
	"github.com/nekoman-hq/neko-cli/internal/log"
	"github.com/nekoman-hq/neko-cli/internal/release"
)

type GoReleaser struct {
	release.ToolBase

	State struct {
		// HEAD before release started
		PreHead string

		// hash of the "chore(neko-release): x.y.z" commit
		ReleaseCommitHash string

		TagName string

		PushedCommit bool
		PushedTag    bool

		RanGoRelease bool
	}
}

//type CommitHash struct {
//	rev string
//}

func (g *GoReleaser) Name() string {
	return "goreleaser"
}

func (g *GoReleaser) Init(_ *config.NekoConfig) error {
	if err := g.RequireBinary(g.Name()); err != nil {
		return err
	}

	if err := runGoreleaserInit(); err != nil {
		return err
	}

	if err := runGoreleaserCheck(); err != nil {
		return err
	}

	return nil
}

func (g *GoReleaser) SupportsSurvey() bool {
	return true
}

func (g *GoReleaser) Release(v *semver.Version) error {
	pre, err := git.Head()
	if err != nil {
		return err
	}
	g.State.PreHead = pre

	if err = g.CreateReleaseCommit(v); err != nil {
		return err
	}

	head, err := git.Head()
	if err != nil {
		return err
	}
	g.State.ReleaseCommitHash = head

	if err := g.CreateGitTag(v); err != nil {
		return err
	}
	g.State.TagName = fmt.Sprintf("v%s", v.String())

	if err := g.PushCommits(); err != nil {
		return err
	}
	g.State.PushedCommit = true

	if err := g.PushGitTag(v); err != nil {
		return err
	}
	g.State.PushedTag = true

	if err := g.runGoReleaserDryRun(); err != nil {
		return err
	}

	if err := g.runGoReleaserRelease(); err != nil {
		return err
	}
	g.State.RanGoRelease = true

	return nil
}

func (g *GoReleaser) RevertRelease() error {
	return g.RevertGitRelease(release.GitReleaseState{
		PreHead:              g.State.PreHead,
		ReleaseHead:          g.State.ReleaseCommitHash,
		TagName:              g.State.TagName,
		PushedCommit:         g.State.PushedCommit,
		PushedTag:            g.State.PushedTag,
		GitHubReleaseTag:     g.State.TagName,
		CreatedGitHubRelease: g.State.RanGoRelease,
	})
}

func runGoreleaserInit() error {
	if _, err := os.Stat(".goreleaser.yaml"); err == nil {
		log.Print(
			log.Init,
			"Skipping goreleaser init, %s already exists",
			log.ColorText(log.ColorCyan, "goreleaser.yml"),
		)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf(
			"failed to check goreleaser.yml: %w",
			err,
		)
	}

	log.V(log.Init,
		fmt.Sprintf("Initializing goreleaser: %s",
			log.ColorText(log.ColorGreen, "goreleaser init"),
		),
	)

	cmd := exec.Command("goreleaser", "init")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"failed to initialize goreleaser: %s: %w", string(output), err,
		)
	}

	log.Print(
		log.Init,
		"\uF00C  Successfully initialized %s",
		log.ColorText(log.ColorCyan, "goreleaser"),
	)

	return nil
}

func runGoreleaserCheck() error {
	log.V(log.Init,
		fmt.Sprintf("Checking goreleaser configuration: %s",
			log.ColorText(log.ColorGreen, "goreleaser check"),
		),
	)

	cmd := exec.Command("goreleaser", "check")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"goreleaser configuration check failed: %s: %w", string(output), err,
		)
	}

	log.Print(
		log.Init,
		"\uF00C Configuration check passed for %s",
		log.ColorText(log.ColorCyan, "goreleaser"),
	)

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
		return fmt.Errorf(
			"GoReleaser release failed: %s: %w", string(output), err,
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
