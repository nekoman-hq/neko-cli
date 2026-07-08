// Package goreleaser includes the goreleaser release-system logic
//
//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package goreleaser

//lint:file-ignore SA1019 V1 executor initialization still receives the legacy config until V2 execution is implemented

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
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/git"
	release2 "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"

	"github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/pkg/log"
)

type GoReleaser struct {
	release2.ToolBase

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

func (g *GoReleaser) Init(_ *config.V1ReleaseConfig) error {
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

func (g *GoReleaser) Execute(ctx *release2.ReleaseExecutionContext) error {
	if ctx == nil {
		return fmt.Errorf("release execution context is missing")
	}
	version, err := semver.NewVersion(ctx.NextVersion)
	if err != nil {
		return fmt.Errorf("invalid next version %q: %w", ctx.NextVersion, err)
	}
	return g.InUnitRoot(ctx, func() error {
		return g.Release(version)
	})
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
	return g.RevertGitRelease(release2.GitReleaseState{
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
	exists, err := goreleaserConfigExists()
	if err != nil {
		return err
	}

	if exists {
		log.PluginPrint(
			log.Init,
			"Skipping goreleaser init, %s already exists",
			log.ColorText(log.ColorCyan, "a goreleaser config file"),
		)
		return nil
	}

	log.PluginV(log.Init,
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

	log.PluginPrint(
		log.Init,
		"\uF00C  Successfully initialized %s",
		log.ColorText(log.ColorCyan, "goreleaser"),
	)

	return nil
}

func goreleaserConfigExists() (bool, error) {
	for _, file := range []string{".goreleaser.yml", ".goreleaser.yaml"} {
		if _, err := os.Stat(file); err == nil {
			return true, nil
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("failed to check %s: %w", file, err)
		}
	}

	return false, nil
}

func runGoreleaserCheck() error {
	log.PluginV(log.Init,
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

	log.PluginPrint(
		log.Init,
		"\uF00C Configuration check passed for %s",
		log.ColorText(log.ColorCyan, "goreleaser"),
	)

	return nil
}

// runGoReleaserDryRun executes goreleaser in dry-run mode
func (g *GoReleaser) runGoReleaserDryRun() error {
	log.PluginV(log.Exec, fmt.Sprintf("Running GoReleaser dry run: %s",
		log.ColorText(log.ColorGreen, "goreleaser release --snapshot --clean")))

	cmd := exec.Command("goreleaser", "release", "--snapshot", "--clean")
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		errors.WriteWarning(
			"GoReleaser dry run failed",
			fmt.Sprintf("This is a warning - proceeding anyway: %s", strings.TrimSpace(string(output))),
		)
		log.PluginPrint(log.Exec, "\u26A0 Dry run failed, but continuing with release")
		return nil
	}

	log.PluginPrint(log.Exec, "\uF00C GoReleaser dry run %s",
		log.ColorText(log.ColorGreen, "successful"))
	return nil
}

// runGoReleaserRelease executes the full goreleaser release
func (g *GoReleaser) runGoReleaserRelease() error {
	log.PluginV(log.Exec, fmt.Sprintf("Running GoReleaser release: %s",
		log.ColorText(log.ColorGreen, "goreleaser release --clean")))

	cmd := exec.Command("goreleaser", "release", "--clean")
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"GoReleaser release failed: %s: %w", string(output), err,
		)
	}

	log.PluginPrint(log.Exec, "\uF00C GoReleaser release %s",
		log.ColorText(log.ColorGreen, "successful"),
	)
	return nil
}

func init() {
	release2.Register(&GoReleaser{})
}
