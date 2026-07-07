// Package release includes all neko cli release logic
package release

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/git"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

type Service struct {
	cfg *config.NekoConfig
}

func NewReleaseService(cfg *config.NekoConfig) *Service {
	return &Service{cfg: cfg}
}

// Run executes the release with the specified release type (patch, minor, major)
func (rs *Service) Run(releaseType Type) error {
	_, _ = git.Current()

	Preflight(rs.cfg)
	version, err := VersionGuard(rs.cfg)
	if err != nil {
		return err
	}

	releaser, err := Get(string(rs.cfg.ReleaseSystem))
	if err != nil {
		return fmt.Errorf(
			"release System Not Found: %w", err,
		)
	}

	log.PluginPrint(log.Exec,
		"Release system detected: %s",
		log.ColorText(log.ColorPurple, releaser.Name()),
	)

	log.PluginPrint(log.Exec,
		"Latest version tag extracted successfully \uF178 %s",
		log.ColorText(log.ColorCyan, version.String()),
	)

	rt, err := ResolveReleaseType(version, releaseType)
	if err != nil {
		return fmt.Errorf(
			"invalid Release Type: %w", err,
		)
	}

	log.PluginPrint(log.Guard, "\uF00C All checks have succeeded. %s", log.ColorText(log.ColorGreen, "Starting release now!"))

	newVersion := NextVersion(version, rt)

	if err := rs.updateConfig(&newVersion); err != nil {
		errors.WriteWarning(
			"Failed to update local config",
			fmt.Sprintf("Updating version in .release.neko.json failed. Attempting to proceed with release: %s", err.Error()))
	}

	mutatingReleaseStarted := true
	if err := releaser.Release(&newVersion); err != nil {
		releaseError := fmt.Errorf("release failed: %w", err)

		if err := rs.updateConfig(version); err != nil {
			log.PluginPrint(log.Guard, "Warning: Failed to revert config: %s", err.Error())
		}

		// Rollback is intentionally only reachable after this service has
		// crossed into the mutating release phase. Earlier planning and guard
		// failures must never trigger destructive git cleanup/reset paths.
		if mutatingReleaseStarted {
			log.PluginPrint(log.Guard, "Encountered error while releasing. Trying to undo changes...")
			if err := releaser.RevertRelease(); err != nil {
				return fmt.Errorf("%w: Failed undoing changes: %w", releaseError, err)
			}
			log.PluginPrint(log.Guard, "Successfully undid changes.")
		}

		return releaseError
	}

	log.PluginPrint(log.Exec, "\uF00C Successfully released version %s",
		log.ColorText(log.ColorCyan, newVersion.String()))

	return nil
}

// GetNewVersion returns what the new version would be for a given release type
func (rs *Service) GetNewVersion(releaseType Type) (*semver.Version, *semver.Version, error) {
	version, err := VersionGuardWithOptions(rs.cfg, VersionGuardOptions{AllowRemoteRefresh: false})
	if err != nil {
		return nil, nil, err
	}

	newVersion := NextVersion(version, releaseType)
	return version, &newVersion, nil
}

func (rs *Service) updateConfig(newVersion *semver.Version) error {
	rs.cfg.Version = newVersion.String()
	return config.SaveConfig(*rs.cfg)
}
