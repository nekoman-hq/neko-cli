// Package goreleaser includes the goreleaser release-system logic
//
//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package goreleaser

//lint:file-ignore SA1019 The legacy init compatibility facade receives the deprecated V1 config.

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	release2 "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"

	"github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/pkg/log"
)

type GoReleaser struct {
	git            v1GitWriter
	rollback       v1Rollback
	process        v1Process
	files          v1FileInspector
	environment    v1Environment
	binaries       v1BinaryLocator
	repositoryRoot string

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

type v1GitWriter interface {
	Head(string) (string, error)
	CreateReleaseCommit(string, *semver.Version) error
	CreateGitTag(string, *semver.Version) error
	PushCommits(string) error
	PushGitTag(string, *semver.Version) error
}

type v1Rollback interface {
	Rollback(string, release2.GitReleaseState) error
}

type v1Process interface {
	Run(string, []string, []string) ([]byte, error)
}

type v1FileInspector interface {
	Exists(string, string) (bool, error)
}

type v1Environment interface {
	Environ() []string
}

type v1BinaryLocator interface {
	Require(string) error
}

func NewV1Executor() *GoReleaser {
	return &GoReleaser{
		git:         release2.NewSystemV1GitWriter(),
		rollback:    release2.NewSystemV1ReleaseRollback(),
		process:     systemV1Process{},
		files:       release2.NewSystemV1FileInspector(),
		environment: systemV1Environment{},
		binaries:    release2.NewSystemV1BinaryLocator(),
	}
}

func (g *GoReleaser) ensureDependencies() {
	if g.git != nil && g.rollback != nil && g.process != nil && g.files != nil && g.environment != nil && g.binaries != nil {
		return
	}
	defaults := NewV1Executor()
	if g.git == nil {
		g.git = defaults.git
	}
	if g.rollback == nil {
		g.rollback = defaults.rollback
	}
	if g.process == nil {
		g.process = defaults.process
	}
	if g.files == nil {
		g.files = defaults.files
	}
	if g.environment == nil {
		g.environment = defaults.environment
	}
	if g.binaries == nil {
		g.binaries = defaults.binaries
	}
}

func (g *GoReleaser) Name() string {
	return "goreleaser"
}

func (g *GoReleaser) Init(_ *config.V1ReleaseConfig) error {
	g.ensureDependencies()
	if err := g.binaries.Require(g.Name()); err != nil {
		return err
	}

	if err := g.runGoreleaserInit(""); err != nil {
		return err
	}

	if err := g.runGoreleaserCheck(""); err != nil {
		return err
	}

	return nil
}

// Execute preserves the legacy Tool method shape.
//
// Deprecated: use Run with V1ExecutorRequest instead.
func (g *GoReleaser) Execute(ctx *release2.ReleaseExecutionContext) error {
	if ctx == nil {
		return fmt.Errorf("release execution context is missing")
	}
	return g.Run(release2.V1ExecutorRequest{Plan: release2.V1ReleasePlan{
		RepositoryRoot: ctx.UnitRoot,
		NextVersion:    ctx.NextVersion,
	}})
}

func (g *GoReleaser) Run(request release2.V1ExecutorRequest) error {
	version, err := semver.NewVersion(request.Plan.NextVersion)
	if err != nil {
		return fmt.Errorf("invalid next version %q: %w", request.Plan.NextVersion, err)
	}
	return g.release(request.Plan.RepositoryRoot, version)
}

// Release preserves the legacy cwd-based direct release method.
//
// Deprecated: use Run with V1ExecutorRequest instead.
func (g *GoReleaser) Release(v *semver.Version) error {
	return g.release("", v)
}

func (g *GoReleaser) release(repositoryRoot string, v *semver.Version) error {
	g.ensureDependencies()
	g.repositoryRoot = repositoryRoot
	pre, err := g.git.Head(repositoryRoot)
	if err != nil {
		return err
	}
	g.State.PreHead = pre

	if err = g.git.CreateReleaseCommit(repositoryRoot, v); err != nil {
		return err
	}

	head, err := g.git.Head(repositoryRoot)
	if err != nil {
		return err
	}
	g.State.ReleaseCommitHash = head

	if err := g.git.CreateGitTag(repositoryRoot, v); err != nil {
		return err
	}
	g.State.TagName = fmt.Sprintf("v%s", v.String())

	if err := g.git.PushCommits(repositoryRoot); err != nil {
		return err
	}
	g.State.PushedCommit = true

	if err := g.git.PushGitTag(repositoryRoot, v); err != nil {
		return err
	}
	g.State.PushedTag = true

	if err := g.runGoReleaserDryRun(repositoryRoot); err != nil {
		return err
	}

	if err := g.runGoReleaserRelease(repositoryRoot); err != nil {
		return err
	}
	g.State.RanGoRelease = true

	return nil
}

// RevertRelease preserves the legacy rollback method name.
//
// Deprecated: use Rollback instead.
func (g *GoReleaser) RevertRelease() error {
	return g.Rollback()
}

func (g *GoReleaser) CompensationState() release2.GitReleaseState {
	return release2.GitReleaseState{
		PreHead:              g.State.PreHead,
		ReleaseHead:          g.State.ReleaseCommitHash,
		TagName:              g.State.TagName,
		PushedCommit:         g.State.PushedCommit,
		PushedTag:            g.State.PushedTag,
		GitHubReleaseTag:     g.State.TagName,
		CreatedGitHubRelease: g.State.RanGoRelease,
	}
}

func (g *GoReleaser) Rollback() error {
	g.ensureDependencies()
	return g.rollback.Rollback(g.repositoryRoot, g.CompensationState())
}

func (g *GoReleaser) runGoreleaserInit(repositoryRoot string) error {
	exists, err := g.goreleaserConfigExists(repositoryRoot)
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

	output, err := g.process.Run(repositoryRoot, []string{"init"}, g.environment.Environ())
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

func (g *GoReleaser) goreleaserConfigExists(repositoryRoot string) (bool, error) {
	for _, file := range []string{".goreleaser.yml", ".goreleaser.yaml"} {
		exists, err := g.files.Exists(repositoryRoot, file)
		if err != nil {
			return false, fmt.Errorf("failed to check %s: %w", file, err)
		}
		if exists {
			return true, nil
		}
	}

	return false, nil
}

func (g *GoReleaser) runGoreleaserCheck(repositoryRoot string) error {
	log.PluginV(log.Init,
		fmt.Sprintf("Checking goreleaser configuration: %s",
			log.ColorText(log.ColorGreen, "goreleaser check"),
		),
	)

	output, err := g.process.Run(repositoryRoot, []string{"check"}, g.environment.Environ())
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
func (g *GoReleaser) runGoReleaserDryRun(repositoryRoot string) error {
	log.PluginV(log.Exec, fmt.Sprintf("Running GoReleaser dry run: %s",
		log.ColorText(log.ColorGreen, "goreleaser release --snapshot --clean")))

	output, err := g.process.Run(repositoryRoot, []string{"release", "--snapshot", "--clean"}, g.environment.Environ())
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
func (g *GoReleaser) runGoReleaserRelease(repositoryRoot string) error {
	log.PluginV(log.Exec, fmt.Sprintf("Running GoReleaser release: %s",
		log.ColorText(log.ColorGreen, "goreleaser release --clean")))

	output, err := g.process.Run(repositoryRoot, []string{"release", "--clean"}, g.environment.Environ())
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

func (g *GoReleaser) ValidateRequirements(ctx *release2.ReleaseExecutionContext) error {
	return release2.ValidateRequirementsForContext(ctx)
}

func (g *GoReleaser) ResolveFiles(ctx *release2.ReleaseExecutionContext) ([]string, error) {
	var compatibility release2.ToolBase
	return compatibility.ResolveFiles(ctx)
}
