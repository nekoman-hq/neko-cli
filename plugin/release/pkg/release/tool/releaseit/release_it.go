// Package releaseit provides functions for release automation.
//
//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package releaseit

//lint:file-ignore SA1019 The legacy init compatibility facade receives the deprecated V1 config.

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	release2 "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"

	"github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/pkg/log"
)

type ReleaseIt struct {
	git            v1GitReader
	rollback       v1Rollback
	process        v1Process
	files          v1FileInspector
	binaries       v1BinaryLocator
	configs        v1ConfigStore
	repositoryRoot string

	packageManager string // "npm" or "bun"

	State struct {
		PreHead           string
		ReleaseCommitHash string

		TagName      string
		PushedCommit bool
		PushedTag    bool

		CreatedGitHubRelease bool
	}
}

type v1GitReader interface {
	Head(string) (string, error)
}

type v1Rollback interface {
	Rollback(string, release2.GitReleaseState) error
}

type v1Process interface {
	Run(string, string, ...string) ([]byte, error)
}

type v1FileInspector interface {
	Exists(string, string) (bool, error)
}

type v1BinaryLocator interface {
	Require(string) error
}

type v1ConfigStore interface {
	Save(string, *Config) error
}

func NewV1Executor() *ReleaseIt {
	return &ReleaseIt{
		git:      release2.NewSystemV1GitWriter(),
		rollback: release2.NewSystemV1ReleaseRollback(),
		process:  newSystemV1Process(),
		files:    release2.NewSystemV1FileInspector(),
		binaries: release2.NewSystemV1BinaryLocator(),
		configs:  systemV1ConfigStore{},
	}
}

func (r *ReleaseIt) ensureDependencies() {
	if r.git != nil && r.rollback != nil && r.process != nil && r.files != nil && r.binaries != nil && r.configs != nil {
		return
	}
	defaults := NewV1Executor()
	if r.git == nil {
		r.git = defaults.git
	}
	if r.rollback == nil {
		r.rollback = defaults.rollback
	}
	if r.process == nil {
		r.process = defaults.process
	}
	if r.files == nil {
		r.files = defaults.files
	}
	if r.binaries == nil {
		r.binaries = defaults.binaries
	}
	if r.configs == nil {
		r.configs = defaults.configs
	}
}

func (r *ReleaseIt) Name() string {
	return "release-it"
}

func (r *ReleaseIt) ensurePackageManager(repositoryRoot string) {
	if r.packageManager == "" {
		r.packageManager = r.detectPackageManagerAt(repositoryRoot)
	}
}

// detectPackageManager checks for lock files to determine the package manager
func (r *ReleaseIt) detectPackageManager() string {
	r.ensureDependencies()
	return r.detectPackageManagerAt("")
}

func (r *ReleaseIt) detectPackageManagerAt(repositoryRoot string) string {
	if exists, _ := r.files.Exists(repositoryRoot, "bun.lock"); exists {
		log.PluginV(log.Init,
			fmt.Sprintf("Detected package manager: %s (found %s)",
				log.ColorText(log.ColorCyan, "bun"),
				log.ColorText(log.ColorYellow, "bun.lockb"),
			),
		)
		return "bun"
	}
	if exists, _ := r.files.Exists(repositoryRoot, "package-lock.json"); exists {
		log.PluginV(log.Init,
			fmt.Sprintf("Detected package manager: %s (found %s)",
				log.ColorText(log.ColorCyan, "npm"),
				log.ColorText(log.ColorYellow, "package-lock.json"),
			),
		)
		return "npm"
	}
	// Default to npm if no lock file is found
	log.PluginV(log.Init,
		fmt.Sprintf("No lock file found, defaulting to %s",
			log.ColorText(log.ColorCyan, "npm"),
		),
	)
	return "npm"
}

// getRunCommand returns the appropriate run command (npx or bunx)
func (r *ReleaseIt) getRunCommand() string {
	if r.packageManager == "bun" {
		return "bunx"
	}
	return "npx"
}

func (r *ReleaseIt) Init(cfg *config.V1ReleaseConfig) error {
	r.ensureDependencies()
	r.ensurePackageManager("")

	if err := r.binaries.Require(r.packageManager); err != nil {
		return err
	}

	if err := r.runReleaseItInit("", cfg); err != nil {
		return err
	}

	if err := r.runReleaseItCheck(""); err != nil {
		return err
	}

	return nil
}

// Execute preserves the legacy Tool method shape.
//
// Deprecated: use Run with V1ExecutorRequest instead.
func (r *ReleaseIt) Execute(ctx *release2.ReleaseExecutionContext) error {
	if ctx == nil {
		return fmt.Errorf("release execution context is missing")
	}
	return r.Run(release2.V1ExecutorRequest{Plan: release2.V1ReleasePlan{
		RepositoryRoot: ctx.UnitRoot,
		NextVersion:    ctx.NextVersion,
	}})
}

func (r *ReleaseIt) Run(request release2.V1ExecutorRequest) error {
	version, err := semver.NewVersion(request.Plan.NextVersion)
	if err != nil {
		return fmt.Errorf("invalid next version %q: %w", request.Plan.NextVersion, err)
	}
	return r.release(request.Plan.RepositoryRoot, version)
}

// Release preserves the legacy cwd-based direct release method.
//
// Deprecated: use Run with V1ExecutorRequest instead.
func (r *ReleaseIt) Release(v *semver.Version) error {
	return r.release("", v)
}

func (r *ReleaseIt) release(repositoryRoot string, v *semver.Version) error {
	r.ensureDependencies()
	r.repositoryRoot = repositoryRoot
	r.ensurePackageManager(repositoryRoot)

	pre, err := r.git.Head(repositoryRoot)
	if err != nil {
		return err
	}
	r.State.PreHead = pre

	if err = r.runReleaseItRelease(repositoryRoot, v); err != nil {
		return err
	}

	head, err := r.git.Head(repositoryRoot)
	if err != nil {
		return err
	}
	r.State.ReleaseCommitHash = head

	r.State.TagName = fmt.Sprintf("v%s", v.String())

	r.State.PushedCommit = true
	r.State.PushedTag = true

	r.State.CreatedGitHubRelease = true

	return nil
}

// RevertRelease preserves the legacy rollback method name.
//
// Deprecated: use Rollback instead.
func (r *ReleaseIt) RevertRelease() error {
	return r.Rollback()
}

func (r *ReleaseIt) CompensationState() release2.GitReleaseState {
	return release2.GitReleaseState{
		PreHead:              r.State.PreHead,
		ReleaseHead:          r.State.ReleaseCommitHash,
		TagName:              r.State.TagName,
		PushedCommit:         r.State.PushedCommit,
		PushedTag:            r.State.PushedTag,
		GitHubReleaseTag:     r.State.TagName,
		CreatedGitHubRelease: r.State.CreatedGitHubRelease,
	}
}

func (r *ReleaseIt) Rollback() error {
	r.ensureDependencies()
	return r.rollback.Rollback(r.repositoryRoot, r.CompensationState())
}

func (r *ReleaseIt) runReleaseItInit(repositoryRoot string, cfg *config.V1ReleaseConfig) error {
	exists, err := r.files.Exists(repositoryRoot, ".release-it.json")
	if err != nil {
		return fmt.Errorf("failed to check .release-it.json: %w", err)
	}
	if exists {
		log.PluginPrint(
			log.Init,
			"Skipping ReleaseIt init, %s already exists",
			log.ColorText(log.ColorCyan, ".release-it.json"),
		)
		return nil
	}

	if exists, _ := r.files.Exists(repositoryRoot, "package.json"); !exists {
		errors.WriteWarning(
			"Project not correctly initialized",
			"No %s found - this doesn't appear to be a Node.js project",
		)
	}

	installCmd := fmt.Sprintf("%s install -D release-it", r.packageManager)
	if r.packageManager == "bun" {
		installCmd = "bun add -D release-it"
	}

	log.PluginV(log.Init,
		fmt.Sprintf("Initializing release-it: %s",
			log.ColorText(log.ColorGreen, installCmd),
		),
	)

	var output []byte
	var commandError error
	if r.packageManager == "bun" {
		output, commandError = r.process.Run(repositoryRoot, "bun", "add", "-D", "release-it")
	} else {
		output, commandError = r.process.Run(repositoryRoot, "npm", "install", "-D", "release-it")
	}

	if commandError != nil {
		return fmt.Errorf(
			"failed to initialize release-it: %s: %w", string(output), commandError,
		)
	}

	rcfg, err := InitDefaultConfig(cfg.ProjectName)
	if err != nil {
		return fmt.Errorf("failed to create default config: %w", err)
	}
	if err := r.configs.Save(repositoryRoot, rcfg); err != nil {
		return fmt.Errorf("failed to save .release-it.json: %w", err)
	}

	log.PluginPrint(
		log.Init,
		"\uF00C  Successfully initialized %s",
		log.ColorText(log.ColorCyan, "release-it"),
	)

	return nil
}

func (r *ReleaseIt) runReleaseItCheck(repositoryRoot string) error {
	runCmd := r.getRunCommand()
	checkCmd := fmt.Sprintf("%s release-it -v", runCmd)

	log.PluginV(log.Init,
		fmt.Sprintf("Verifying release-it installation: %s",
			log.ColorText(log.ColorGreen, checkCmd),
		),
	)

	output, err := r.process.Run(repositoryRoot, runCmd, "release-it", "-v")
	if err != nil {
		return fmt.Errorf(
			"failed to verify release-it installation: %s: %w", string(output), err,
		)
	}
	log.PluginPrint(
		log.Exec,
		"\uF00C  Successfully verified %s installation ( version: %s )",
		log.ColorText(log.ColorCyan, "release-it"),
		log.ColorText(log.ColorGreen, string(output)),
	)

	return nil
}

func (r *ReleaseIt) runReleaseItRelease(repositoryRoot string, v *semver.Version) error {
	versionStr := v.String()
	runCmd := r.getRunCommand()
	releaseCmd := fmt.Sprintf("%s release-it %s --ci --no-git.requireCleanWorkingDir", runCmd, versionStr)

	log.PluginV(log.Exec,
		fmt.Sprintf("Running release-it: %s",
			log.ColorText(log.ColorGreen, releaseCmd),
		),
	)

	output, err := r.process.Run(repositoryRoot, runCmd, "release-it", versionStr, "--ci", "--no-git.requireCleanWorkingDir")
	if err != nil {
		return fmt.Errorf("release failed: %s\nOutput: %s", err.Error(), string(output))
	}
	return nil
}

func (r *ReleaseIt) ValidateRequirements(ctx *release2.ReleaseExecutionContext) error {
	return release2.ValidateRequirementsForContext(ctx)
}

func (r *ReleaseIt) ResolveFiles(ctx *release2.ReleaseExecutionContext) ([]string, error) {
	var compatibility release2.ToolBase
	return compatibility.ResolveFiles(ctx)
}
