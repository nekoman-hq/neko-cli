// Package legacyrequirements validates the environment and files required by
// the retained V1 release format.
package legacyrequirements

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	coreconfig "github.com/nekoman-hq/neko-cli/pkg/config"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// Validate checks the retained V1 token and release-tool configuration-file
// contract at an explicit repository root.
//
//nolint:staticcheck // The boundary intentionally preserves the deprecated V1 input contract.
func Validate(repositoryRoot string, cfg *releaseconfig.V1ReleaseConfig) error {
	return validate(repositoryRoot, cfg, true)
}

// ValidateForInspection applies the same retained V1 requirements without
// emitting lifecycle progress for a deterministic read-only query.
func ValidateForInspection(repositoryRoot string, cfg *releaseconfig.V1ReleaseConfig) error {
	return validate(repositoryRoot, cfg, false)
}

func validate(repositoryRoot string, cfg *releaseconfig.V1ReleaseConfig, reportProgress bool) error {
	if cfg == nil {
		return fmt.Errorf("release configuration is missing")
	}

	if reportProgress {
		log.PluginV(
			log.Config,
			"Validating release requirements for %s",
			log.ColorText(log.ColorCyan, string(cfg.ReleaseSystem)),
		)
	}

	if _, err := coreconfig.GetPAT(); err != nil {
		return err
	}

	requiredFiles, err := configCandidates(string(cfg.ReleaseSystem))
	if err != nil {
		return err
	}
	for _, file := range requiredFiles {
		target := filepath.Join(repositoryRoot, file)
		if _, err := os.Stat(target); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("failed to check %s: %w", target, err)
		}
	}

	if len(requiredFiles) == 1 {
		return fmt.Errorf(
			"required %s configuration missing: %s not found",
			cfg.ReleaseSystem,
			requiredFiles[0],
		)
	}

	return fmt.Errorf(
		"required %s configuration missing: none of %s were found",
		cfg.ReleaseSystem,
		quotedFiles(requiredFiles),
	)
}

func configCandidates(releaseSystem string) ([]string, error) {
	identity, err := releasetool.ParseIdentity(releaseSystem)
	if err != nil {
		return nil, fmt.Errorf("unknown release system: %s", releaseSystem)
	}
	return releasetool.ConfigCandidates(identity)
}

func quotedFiles(files []string) string {
	quoted := make([]string, len(files))
	for index, file := range files {
		quoted[index] = fmt.Sprintf("%q", file)
	}
	return strings.Join(quoted, ", ")
}
