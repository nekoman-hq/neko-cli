//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package release

//lint:file-ignore SA1019 V1 compatibility release paths intentionally use deprecated V1 APIs during migration

import (
	"fmt"
	"os"
	"path/filepath"
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

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to resolve current working directory: %w", err)
	}

	return validateRequirementsForExecutor(string(cfg.ReleaseSystem), cwd, true)
}

// ValidateRequirementsForContext checks executor requirements relative to the
// release unit root, not the caller's current working directory.
func ValidateRequirementsForContext(ctx *ReleaseExecutionContext) error {
	if ctx == nil {
		return fmt.Errorf("release execution context is missing")
	}

	log.PluginV(
		log.Config,
		"Validating release requirements for %s in %s",
		log.ColorText(log.ColorCyan, ctx.Executor),
		log.ColorText(log.ColorGreen, ctx.UnitRoot),
	)

	return validateRequirementsForExecutor(ctx.Executor, ctx.UnitRoot, !ctx.DryRun)
}

func validateRequirementsForExecutor(executor, unitRoot string, requireToken bool) error {
	if requireToken {
		if _, err := coreconfig.GetPAT(); err != nil {
			return err
		}
	}

	requiredFiles, err := requiredReleaseSystemFiles(executor)
	if err != nil {
		return err
	}

	for _, file := range requiredFiles {
		target := filepath.Join(unitRoot, file)
		if _, err := os.Stat(target); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("failed to check %s: %w", target, err)
		}
	}

	if len(requiredFiles) == 1 {
		return fmt.Errorf(
			"required %s configuration missing: %s not found",
			executor,
			requiredFiles[0],
		)
	}

	return fmt.Errorf(
		"required %s configuration missing: none of %s were found",
		executor,
		joinQuotedFiles(requiredFiles),
	)
}

func requiredReleaseSystemFiles(executor string) ([]string, error) {
	switch releaseconfig.ExecutorType(executor) {
	case releaseconfig.ExecutorReleaseIt:
		return []string{releaseItConfigFile}, nil
	case releaseconfig.ExecutorJReleaser:
		return []string{jReleaserConfigFile}, nil
	case releaseconfig.ExecutorGoReleaser:
		return []string{goReleaserConfigFileYML, goReleaserConfigFileYAML}, nil
	default:
		return nil, fmt.Errorf("unknown release system: %s", executor)
	}
}

func joinQuotedFiles(files []string) string {
	quoted := make([]string, len(files))
	for i, file := range files {
		quoted[i] = fmt.Sprintf("%q", file)
	}

	return strings.Join(quoted, ", ")
}
