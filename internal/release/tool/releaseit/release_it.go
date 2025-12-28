// Package releaseit provides functions for release automation.
package releaseit

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/log"
	"github.com/nekoman-hq/neko-cli/internal/release"
)

type ReleaseIt struct {
	release.ToolBase
}

func (r *ReleaseIt) Name() string {
	return "release-it"
}

func (r *ReleaseIt) Init(cfg *config.NekoConfig) error {
	r.RequireBinary("npm")
	r.runReleaseItInit()
	r.runReleaseItCheck()

	return nil
}

func (r *ReleaseIt) Release(v *semver.Version) error {
	if err := r.runReleaseItDryRun(v); err != nil {
		return err
	}

	if err := r.runReleaseItRelease(v); err != nil {
		return err
	}
	return nil
}

func (r *ReleaseIt) Survey(v *semver.Version) (release.Type, error) {
	return release.NekoSurvey(v)
}

func (r *ReleaseIt) SupportsSurvey() bool {
	return true
}

func (r *ReleaseIt) runReleaseItInit() {
	if _, err := os.Stat(".release-it.json"); err == nil {
		log.Print(
			log.Init,
			"Skipping ReleaseIt init, %s already exists",
			log.ColorText(log.ColorCyan, ".release-it.json"),
		)
		return
	} else if !os.IsNotExist(err) {
		errors.Fatal(
			"Failed to check .release-it.json",
			err.Error(),
			errors.ErrFileAccess,
		)
		return
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
		errors.Fatal(
			"Failed to initialize release-it",
			fmt.Sprintf("Command failed: %s\nOutput: %s", err.Error(), string(output)),
			errors.ErrDependencyMissing,
		)
	}

	log.Print(
		log.Init,
		"\uF00C  Successfully initialized %s",
		log.ColorText(log.ColorCyan, "release-it"),
	)
}

func (r *ReleaseIt) runReleaseItCheck() {
	log.V(log.Init,
		fmt.Sprintf("Verifying release-it installation: %s",
			log.ColorText(log.ColorGreen, "npx release-it -v"),
		),
	)
	cmd := exec.Command("npx", "release-it", "-v")
	output, err := cmd.CombinedOutput()
	if err != nil {
		errors.Fatal(
			"Failed to verify release-it installation",
			fmt.Sprintf("Command failed: %s\nOutput: %s", err.Error(), string(output)),
			errors.ErrDependencyMissing,
		)
	}
	log.Print(
		log.Init,
		"\uF00C  Successfully verified %s installation (version: %s)",
		log.ColorText(log.ColorCyan, "release-it"),
		log.ColorText(log.ColorGreen, string(output)),
	)
}

func (r *ReleaseIt) runReleaseItDryRun(v *semver.Version) error {
	versionStr := v.String()
	log.V(log.Init,
		fmt.Sprintf("Running release-it dry-run: %s",
			log.ColorText(log.ColorGreen, fmt.Sprintf("npx release-it %s --ci --dry-run", versionStr)),
		),
	)
	cmd := exec.Command("npx", "release-it", versionStr, "--ci", "--dry-run")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dry-run failed: %s\nOutput: %s", err.Error(), string(output))
	}
	log.Print(
		log.Init,
		"\uF00C  Dry-run successful for version %s",
		log.ColorText(log.ColorCyan, versionStr),
	)
	return nil
}

func (r *ReleaseIt) runReleaseItRelease(v *semver.Version) error {
	versionStr := v.String()
	log.V(log.Init,
		fmt.Sprintf("Running release-it: %s",
			log.ColorText(log.ColorGreen, fmt.Sprintf("npx release-it %s --ci", versionStr)),
		),
	)
	cmd := exec.Command("npx", "release-it", versionStr, "--ci")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("release failed: %s\nOutput: %s", err.Error(), string(output))
	}
	log.Print(
		log.Init,
		"\uF00C  Successfully released version %s",
		log.ColorText(log.ColorCyan, versionStr),
	)
	return nil
}

func init() {
	release.Register(&ReleaseIt{})
}
