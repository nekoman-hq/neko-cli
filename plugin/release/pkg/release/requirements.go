//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package release

//lint:file-ignore SA1019 V1 compatibility release paths intentionally use deprecated V1 APIs during migration

import (
	"fmt"
	"os"
	"strings"

	coreconfig "github.com/nekoman-hq/neko-cli/pkg/config"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const (
	releaseItConfigFile      = ".release-it.json"
	jReleaserConfigFile      = "jreleaser.yml"
	goReleaserConfigFileYML  = ".goreleaser.yml"
	goReleaserConfigFileYAML = ".goreleaser.yaml"
)

// ValidateRequirements checks that the required environment and release-system
// specific files exist before validation or release execution continues.
func ValidateRequirements(cfg *releaseconfig.V1ReleaseConfig) error {
	if cfg == nil {
		return fmt.Errorf("release configuration is missing")
	}

	log.PluginV(
		log.Config,
		"Validating release requirements for %s",
		log.ColorText(log.ColorCyan, string(cfg.ReleaseSystem)),
	)

	if _, err := coreconfig.GetPAT(); err != nil {
		return err
	}

	requiredFiles, err := requiredReleaseSystemFiles(cfg.ReleaseSystem)
	if err != nil {
		return err
	}

	for _, file := range requiredFiles {
		if _, err := os.Stat(file); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("failed to check %s: %w", file, err)
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
		joinQuotedFiles(requiredFiles),
	)
}

func requiredReleaseSystemFiles(system releaseconfig.V1ReleaseSystem) ([]string, error) {
	switch system {
	case releaseconfig.V1ReleaseTypeReleaseIt:
		return []string{releaseItConfigFile}, nil
	case releaseconfig.V1ReleaseTypeJReleaser:
		return []string{jReleaserConfigFile}, nil
	case releaseconfig.V1ReleaseTypeGoReleaser:
		return []string{goReleaserConfigFileYML, goReleaserConfigFileYAML}, nil
	default:
		return nil, fmt.Errorf("unknown release system: %s", system)
	}
}

func joinQuotedFiles(files []string) string {
	quoted := make([]string, len(files))
	for i, file := range files {
		quoted[i] = fmt.Sprintf("%q", file)
	}

	return strings.Join(quoted, ", ")
}
