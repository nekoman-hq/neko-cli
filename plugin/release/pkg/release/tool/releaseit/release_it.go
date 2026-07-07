// Package releaseit provides functions for release automation.
//
//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package releaseit

//lint:file-ignore SA1019 V1 executor initialization still receives the legacy config until V2 execution is implemented

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/git"
	release2 "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"

	"github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/pkg/log"
)

type ReleaseIt struct {
	release2.ToolBase

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

func (r *ReleaseIt) Name() string {
	return "release-it"
}

func (r *ReleaseIt) ensurePackageManager() {
	if r.packageManager == "" {
		r.packageManager = r.detectPackageManager()
	}
}

// detectPackageManager checks for lock files to determine the package manager
func (r *ReleaseIt) detectPackageManager() string {
	if _, err := os.Stat("bun.lock"); err == nil {
		log.PluginV(log.Init,
			fmt.Sprintf("Detected package manager: %s (found %s)",
				log.ColorText(log.ColorCyan, "bun"),
				log.ColorText(log.ColorYellow, "bun.lockb"),
			),
		)
		return "bun"
	}
	if _, err := os.Stat("package-lock.json"); err == nil {
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
	r.ensurePackageManager()

	if err := r.RequireBinary(r.packageManager); err != nil {
		return err
	}

	if err := r.runReleaseItInit(cfg); err != nil {
		return err
	}

	if err := r.runReleaseItCheck(); err != nil {
		return err
	}

	return nil
}

func (r *ReleaseIt) Release(v *semver.Version) error {
	r.ensurePackageManager()

	pre, err := git.Head()
	if err != nil {
		return err
	}
	r.State.PreHead = pre

	if err = r.runReleaseItRelease(v); err != nil {
		return err
	}

	head, err := git.Head()
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

func (r *ReleaseIt) RevertRelease() error {
	return r.RevertGitRelease(release2.GitReleaseState{
		PreHead:              r.State.PreHead,
		ReleaseHead:          r.State.ReleaseCommitHash,
		TagName:              r.State.TagName,
		PushedCommit:         r.State.PushedCommit,
		PushedTag:            r.State.PushedTag,
		GitHubReleaseTag:     r.State.TagName,
		CreatedGitHubRelease: r.State.CreatedGitHubRelease,
	})
}

func (r *ReleaseIt) runReleaseItInit(cfg *config.V1ReleaseConfig) error {
	if _, err := os.Stat(".release-it.json"); err == nil {
		log.PluginPrint(
			log.Init,
			"Skipping ReleaseIt init, %s already exists",
			log.ColorText(log.ColorCyan, ".release-it.json"),
		)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf(
			"failed to check .release-it.json: %w", err,
		)
	}

	if _, err := os.Stat("package.json"); os.IsNotExist(err) {
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

	var cmd *exec.Cmd
	if r.packageManager == "bun" {
		cmd = exec.Command("bun", "add", "-D", "release-it")
	} else {
		cmd = exec.Command("npm", "install", "-D", "release-it")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"failed to initialize release-it: %s: %w", string(output), err,
		)
	}

	rcfg, err := InitDefaultConfig(cfg.ProjectName)
	if err != nil {
		return fmt.Errorf("failed to create default config: %w", err)
	}
	if err := SaveConfig(rcfg); err != nil {
		return fmt.Errorf("failed to save .release-it.json: %w", err)
	}

	log.PluginPrint(
		log.Init,
		"\uF00C  Successfully initialized %s",
		log.ColorText(log.ColorCyan, "release-it"),
	)

	return nil
}

func (r *ReleaseIt) runReleaseItCheck() error {
	runCmd := r.getRunCommand()
	checkCmd := fmt.Sprintf("%s release-it -v", runCmd)

	log.PluginV(log.Init,
		fmt.Sprintf("Verifying release-it installation: %s",
			log.ColorText(log.ColorGreen, checkCmd),
		),
	)

	cmd := exec.Command(runCmd, "release-it", "-v")
	output, err := cmd.CombinedOutput()
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

func (r *ReleaseIt) runReleaseItRelease(v *semver.Version) error {
	versionStr := v.String()
	runCmd := r.getRunCommand()
	releaseCmd := fmt.Sprintf("%s release-it %s --ci --no-git.requireCleanWorkingDir", runCmd, versionStr)

	log.PluginV(log.Exec,
		fmt.Sprintf("Running release-it: %s",
			log.ColorText(log.ColorGreen, releaseCmd),
		),
	)

	cmd := exec.Command(runCmd, "release-it", versionStr, "--ci", "--no-git.requireCleanWorkingDir")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("release failed: %s\nOutput: %s", err.Error(), string(output))
	}
	return nil
}

func init() {
	release2.Register(&ReleaseIt{})
}
