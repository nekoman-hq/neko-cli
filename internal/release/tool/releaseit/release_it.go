// Package releaseit provides functions for release automation.
package releaseit

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/git"
	"github.com/nekoman-hq/neko-cli/internal/log"
	"github.com/nekoman-hq/neko-cli/internal/release"
)

type ReleaseIt struct {
	release.ToolBase

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

func (r *ReleaseIt) Init(cfg *config.NekoConfig) error {
	if err := r.RequireBinary("npm"); err != nil {
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
	pre, err := git.Head()
	if err != nil {
		return err
	}
	r.State.PreHead = pre

	if err := r.runReleaseItRelease(v); err != nil {
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

func (r *ReleaseIt) Survey(v *semver.Version) (release.Type, error) {
	return release.NekoSurvey(v)
}

func (r *ReleaseIt) SupportsSurvey() bool {
	return true
}

func (r *ReleaseIt) RevertRelease() error {
	return r.RevertGitRelease(release.GitReleaseState{
		PreHead:              r.State.PreHead,
		ReleaseHead:          r.State.ReleaseCommitHash,
		TagName:              r.State.TagName,
		PushedCommit:         r.State.PushedCommit,
		PushedTag:            r.State.PushedTag,
		GitHubReleaseTag:     r.State.TagName,
		CreatedGitHubRelease: r.State.CreatedGitHubRelease,
	})
}

func (r *ReleaseIt) runReleaseItInit(cfg *config.NekoConfig) error {
	if _, err := os.Stat(".release-it.json"); err == nil {
		log.Print(
			log.Init,
			"Skipping ReleaseIt init, %s already exists",
			log.ColorText(log.ColorCyan, ".release-it.json"),
		)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf(
			"Failed to check .release-it.json: %w", err,
		)
	}

	if _, err := os.Stat("package.json"); os.IsNotExist(err) {
		errors.Warning(
			"Project not correctly initialized",
			"No %s found - this doesn't appear to be a Node.js project",
		)
	}

	log.V(log.Init,
		fmt.Sprintf("Initializing release-it: %s",
			log.ColorText(log.ColorGreen, "npm install -D release-it"),
		),
	)

	cmd := exec.Command("npm", "install", "-D", "release-it")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"Failed to initialize release-it: %s: %w", string(output), err,
		)
	}

	rcfg, err := InitDefaultConfig(cfg.ProjectName)
	if err != nil {
		return fmt.Errorf("Failed to create default config: %w", err)
	}
	if err := SaveConfig(rcfg); err != nil {
		return fmt.Errorf("Failed to save .release-it.json: %w", err)
	}

	log.Print(
		log.Init,
		"\uF00C  Successfully initialized %s",
		log.ColorText(log.ColorCyan, "release-it"),
	)

	return nil
}

func (r *ReleaseIt) runReleaseItCheck() error {
	log.V(log.Init,
		fmt.Sprintf("Verifying release-it installation: %s",
			log.ColorText(log.ColorGreen, "npx release-it -v"),
		),
	)
	cmd := exec.Command("npx", "release-it", "-v")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"Failed to verify release-it installation: %s: %w", string(output), err,
		)
	}
	log.Print(
		log.Release,
		"\uF00C  Successfully verified %s installation ( version: %s )",
		log.ColorText(log.ColorCyan, "release-it"),
		log.ColorText(log.ColorGreen, string(output)),
	)

	return nil
}

func (r *ReleaseIt) runReleaseItRelease(v *semver.Version) error {
	versionStr := v.String()
	log.V(log.Release,
		fmt.Sprintf("Running release-it: %s",
			log.ColorText(log.ColorGreen, fmt.Sprintf("npx release-it %s --ci --no-git.requireCleanWorkingDir", versionStr)),
		),
	)
	cmd := exec.Command("npx", "release-it", versionStr, "--ci", "--no-git.requireCleanWorkingDir")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("release failed: %s\nOutput: %s", err.Error(), string(output))
	}
	return nil
}

func init() {
	release.Register(&ReleaseIt{})
}
