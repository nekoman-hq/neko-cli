// Package jreleaser includes the jreleaser release-system logic
//
//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package jreleaser

//lint:file-ignore SA1019 The legacy init compatibility facade receives the deprecated V1 config.

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      24.12.2025
*/

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
	config2 "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	release2 "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"

	"github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/pkg/log"
)

type JReleaser struct {
	git            v1GitWriter
	rollback       v1Rollback
	commands       v1JReleaserCommand
	files          v1FileInspector
	configs        v1ConfigStore
	clock          v1Clock
	binaries       v1BinaryLocator
	repositoryRoot string

	State struct {
		PreHead           string
		ReleaseCommitHash string
		TagName           string
		RanJRelease       bool
		PushedCommit      bool
	}
}

type v1GitWriter interface {
	Head(string) (string, error)
	CreateReleaseCommit(string, *semver.Version) error
	PushCommits(string) error
}

type v1Rollback interface {
	Rollback(string, release2.GitReleaseState) error
}

type v1JReleaserCommand interface {
	Run(string, ...string) ([]byte, error)
}

type v1FileInspector interface {
	Exists(string, string) (bool, error)
}

type v1ConfigStore interface {
	Load(string) (*Config, error)
	Save(string, *Config) error
}

type v1Clock interface {
	Year() int
}

type v1BinaryLocator interface {
	Require(string) error
}

func NewV1Executor() *JReleaser {
	return &JReleaser{
		git:      release2.NewSystemV1GitWriter(),
		rollback: release2.NewSystemV1ReleaseRollback(),
		commands: newSystemV1JReleaserCommand(),
		files:    release2.NewSystemV1FileInspector(),
		configs:  systemV1ConfigStore{},
		clock:    systemV1Clock{},
		binaries: release2.NewSystemV1BinaryLocator(),
	}
}

func (j *JReleaser) ensureDependencies() {
	if j.git != nil && j.rollback != nil && j.commands != nil && j.files != nil && j.configs != nil && j.clock != nil && j.binaries != nil {
		return
	}
	defaults := NewV1Executor()
	if j.git == nil {
		j.git = defaults.git
	}
	if j.rollback == nil {
		j.rollback = defaults.rollback
	}
	if j.commands == nil {
		j.commands = defaults.commands
	}
	if j.files == nil {
		j.files = defaults.files
	}
	if j.configs == nil {
		j.configs = defaults.configs
	}
	if j.clock == nil {
		j.clock = defaults.clock
	}
	if j.binaries == nil {
		j.binaries = defaults.binaries
	}
}

func (j *JReleaser) Name() string {
	return string(releasetool.JReleaser)
}

func (j *JReleaser) Init(cfg *config2.V1ReleaseConfig) error {
	j.ensureDependencies()
	log.PluginV(log.Init, fmt.Sprintf("Initializing %s for project %s@%s",
		log.ColorText(log.ColorGreen, j.Name()),
		cfg.ProjectName,
		cfg.Version,
	))

	if err := j.binaries.Require(j.Name()); err != nil {
		return err
	}
	if err := j.runJReleaserInit("", cfg); err != nil {
		return err
	}
	if err := j.runJReleaserCheck(""); err != nil {
		return err
	}

	log.PluginPrint(log.Init, "\uF00C Initialization complete for %s", log.ColorText(log.ColorCyan, j.Name()))
	return nil
}

// Execute preserves the legacy Tool method shape.
//
// Deprecated: use Run with V1ExecutorRequest instead.
func (j *JReleaser) Execute(ctx *release2.ReleaseExecutionContext) error {
	if ctx == nil {
		return fmt.Errorf("release execution context is missing")
	}
	return j.Run(release2.V1ExecutorRequest{Plan: release2.V1ReleasePlan{
		RepositoryRoot: ctx.UnitRoot,
		NextVersion:    ctx.NextVersion,
	}})
}

func (j *JReleaser) Run(request release2.V1ExecutorRequest) error {
	version, err := semver.NewVersion(request.Plan.NextVersion)
	if err != nil {
		return fmt.Errorf("invalid next version %q: %w", request.Plan.NextVersion, err)
	}
	return j.release(request.Plan.RepositoryRoot, version)
}

// Release preserves the legacy cwd-based direct release method.
//
// Deprecated: use Run with V1ExecutorRequest instead.
func (j *JReleaser) Release(v *semver.Version) error {
	return j.release("", v)
}

func (j *JReleaser) release(repositoryRoot string, v *semver.Version) error {
	j.ensureDependencies()
	j.repositoryRoot = repositoryRoot
	pre, err := j.git.Head(repositoryRoot)

	if err != nil {
		return err
	}
	j.State.PreHead = pre

	if err = j.syncJReleaser(repositoryRoot, v); err != nil {
		return err
	}

	if err = j.git.CreateReleaseCommit(repositoryRoot, v); err != nil {
		return err
	}

	head, err := j.git.Head(repositoryRoot)
	if err != nil {
		return err
	}
	j.State.ReleaseCommitHash = head

	if err = j.git.PushCommits(repositoryRoot); err != nil {
		return err
	}
	j.State.PushedCommit = true

	if err = j.runJReleaserDryRun(repositoryRoot); err != nil {
		return err
	}

	if err = j.runJReleaserRelease(repositoryRoot); err != nil {
		return err
	}
	j.State.TagName = fmt.Sprintf("v%s", v.String())
	j.State.RanJRelease = true

	return nil
}

// RevertRelease preserves the legacy rollback method name.
//
// Deprecated: use Rollback instead.
func (j *JReleaser) RevertRelease() error {
	return j.Rollback()
}

func (j *JReleaser) CompensationState() release2.GitReleaseState {
	return release2.GitReleaseState{
		PreHead:              j.State.PreHead,
		ReleaseHead:          j.State.ReleaseCommitHash,
		PushedCommit:         j.State.PushedCommit,
		TagName:              j.State.TagName,
		PushedTag:            j.State.RanJRelease,
		GitHubReleaseTag:     j.State.TagName,
		CreatedGitHubRelease: j.State.RanJRelease,
	}
}

func (j *JReleaser) Rollback() error {
	j.ensureDependencies()
	return j.rollback.Rollback(j.repositoryRoot, j.CompensationState())
}

func (j *JReleaser) runJReleaserInit(repositoryRoot string, cfg *config2.V1ReleaseConfig) error {
	log.PluginV(log.Init, "Generating JReleaser configuration...")

	exists, err := j.files.Exists(repositoryRoot, releasetool.JReleaserConfigFile)
	if err != nil {
		return fmt.Errorf("failed to check jreleaser.yml: %w", err)
	}
	if exists {
		log.PluginPrint(
			log.Init,
			"Skipping jreleaser init, %s already exists",
			log.ColorText(log.ColorCyan, "jreleaser.yml"),
		)
		return nil
	}

	jcfg := &Config{
		Project: Project{
			Name:    cfg.ProjectName,
			Version: cfg.Version,
			Authors: &[]string{"Authors here..."},
			License: "Proprietary",
			Languages: ProjectLanguages{
				Java: JavaLanguage{
					GroupID: fmt.Sprintf("at.%s", cfg.ProjectName),
					Version: "25",
				},
			},
			InceptionYear: strconv.Itoa(j.clock.Year()),
		},
		Release: Release{
			Github: GithubRelease{
				Overwrite:   false,
				Owner:       cfg.ProjectOwner,
				Name:        cfg.ProjectName,
				TagName:     "v{{projectVersion}}",
				ReleaseName: fmt.Sprintf("%s@{{projectVersion}}", cfg.ProjectName),
				Changelog: Changelog{
					Enabled:          true,
					Sort:             "DESC",
					SkipMergeCommits: true,
					Formatted:        "ALWAYS",
					Preset:           "gitmoji",
					Contributors: &Contributors{
						Enabled: false,
					},
					Append: &ChangelogAppend{
						Enabled: true,
						Title:   "## [{{tagName}}]",
						Target:  "CHANGELOG.md",
					},
					IncludeLabels: &[]string{
						"feature", "feat", "fix", "refactor", "improvement", "chore", "test", "docs", "hotfix",
					},
					Labelers: &[]Labeler{
						{Label: "feat", Title: "regex:feat", Order: 1},
						{Label: "feature", Title: "regex:feature", Order: 1},
						{Label: "fix", Title: "regex:fix", Order: 2},
						{Label: "bug", Title: "regex:bug", Order: 2},
						{Label: "refactor", Title: "regex:refactor", Order: 3},
						{Label: "improvement", Title: "regex:improvement", Order: 3},
						{Label: "docs", Title: "regex:docs", Order: 4},
						{Label: "chore", Title: "regex:chore", Order: 5},
						{Label: "test", Title: "regex:test", Order: 6},
						{Label: "hotfix", Title: "regex:hotfix", Order: 7},
					},
					Categories: &[]Category{
						{Title: "Features", Key: "features", Labels: []string{"feat", "feature"}, Order: 1},
						{Title: "Bug Fixes", Key: "fixes", Labels: []string{"fix", "bug"}, Order: 2},
						{Title: "Refactoring", Key: "refactor", Labels: []string{"refactor", "improvement"}, Order: 3},
						{Title: "Documentation", Key: "docs", Labels: []string{"docs"}, Order: 4},
						{Title: "Chores", Key: "chore", Labels: []string{"chore"}, Order: 5},
						{Title: "Tests", Key: "test", Labels: []string{"test"}, Order: 6},
						{Title: "Hotfixes", Key: "hotfix", Labels: []string{"hotfix"}, Order: 7},
					},
				},
			},
		},
	}

	if err := j.configs.Save(repositoryRoot, jcfg); err != nil {
		return fmt.Errorf(
			"configuration write failed: %w", err,
		)
	}
	log.PluginPrint(log.Init, "\uF00C JReleaser configuration generated for %s", log.ColorText(log.ColorCyan, cfg.ProjectName))

	return nil
}

func (j *JReleaser) runJReleaserCheck(repositoryRoot string) error {
	log.PluginV(log.Init,
		"Checking JReleaser configuration: %s",
		log.ColorText(log.ColorGreen, "jreleaser config"),
	)

	output, err := j.commands.Run(repositoryRoot, "config")
	if err != nil {
		return fmt.Errorf(
			"JReleaser configuration check failed: %s: %w", string(output), err,
		)
	}

	log.PluginPrint(
		log.Init,
		"\uF00C Configuration check passed for %s",
		log.ColorText(log.ColorCyan, "jreleaser"),
	)

	return nil
}

func (j *JReleaser) syncJReleaser(repositoryRoot string, v *semver.Version) error {
	log.PluginV(log.Exec,
		fmt.Sprintf("Syncing JReleaser configuration with version %s",
			log.ColorText(log.ColorCyan, v.String()),
		),
	)

	exists, err := j.files.Exists(repositoryRoot, releasetool.JReleaserConfigFile)
	if err != nil {
		return fmt.Errorf("failed to check jreleaser.yml: %w", err)
	}
	if !exists {
		return fmt.Errorf("jreleaser.yml not found")
	}

	jcfg, err := j.configs.Load(repositoryRoot)
	if err != nil {
		return fmt.Errorf(
			"configuration serialization failed: %w", err,
		)
	}

	jcfg.Project.Version = v.String()

	if err := j.configs.Save(repositoryRoot, jcfg); err != nil {
		return fmt.Errorf(
			"configuration write failed: %w", err,
		)
	}

	log.PluginPrint(log.Exec,
		"\uF00C JReleaser version updated to %s",
		log.ColorText(log.ColorGreen, v.String()),
	)

	return nil
}

// runJReleaserDryRun executes JReleaser in dry-run mode
func (j *JReleaser) runJReleaserDryRun(repositoryRoot string) error {
	args := []string{"full-release", "--dry-run"}

	log.PluginV(
		log.Exec,
		fmt.Sprintf(
			"Running JReleaser dry run: %s",
			log.ColorText(log.ColorGreen, "jreleaser "+strings.Join(args, " ")),
		),
	)

	output, err := j.commands.Run(repositoryRoot, args...)
	if err != nil {
		errors.WriteWarning(
			"JReleaser dry run failed",
			fmt.Sprintf(
				"This is a warning - proceeding anyway: %s",
				strings.TrimSpace(string(output)),
			),
		)
		log.PluginPrint(log.Exec, "\u26A0 Dry run failed, but continuing with release")
		return nil
	}

	log.PluginPrint(
		log.Exec,
		"\uF00C JReleaser dry run %s",
		log.ColorText(log.ColorGreen, "successful"),
	)
	return nil
}

// runJReleaserRelease executes the full jreleaser release
func (j *JReleaser) runJReleaserRelease(repositoryRoot string) error {
	args := []string{"full-release"}

	log.PluginV(
		log.Exec,
		fmt.Sprintf(
			"Running JReleaser release: %s",
			log.ColorText(log.ColorGreen, "jreleaser "+strings.Join(args, " ")),
		),
	)

	output, err := j.commands.Run(repositoryRoot, args...)
	if err != nil {
		return fmt.Errorf(
			"JReleaser release failed: %s: %w", string(output), err,
		)
	}

	log.PluginPrint(
		log.Exec,
		"\uF00C JReleaser release %s",
		log.ColorText(log.ColorGreen, "successful"),
	)
	return nil
}

func (j *JReleaser) ValidateRequirements(ctx *release2.ReleaseExecutionContext) error {
	return release2.ValidateRequirementsForContext(ctx)
}

func (j *JReleaser) ResolveFiles(ctx *release2.ReleaseExecutionContext) ([]string, error) {
	var compatibility release2.ToolBase
	return compatibility.ResolveFiles(ctx)
}
