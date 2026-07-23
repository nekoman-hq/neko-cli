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
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/legacyrequirements"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const (
	releaseItConfigFile      = releasetool.ReleaseItConfigFile
	jReleaserConfigFile      = releasetool.JReleaserConfigFile
	goReleaserConfigFileYML  = releasetool.GoReleaserConfigFileYML
	goReleaserConfigFileYAML = releasetool.GoReleaserConfigFileYAML
)

// ValidateRequirements checks that the required environment and release-system
// specific files exist before validation or release execution continues.
func ValidateRequirements(cfg *releaseconfig.V1ReleaseConfig) error {
	if cfg == nil {
		return fmt.Errorf("release configuration is missing")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to resolve current working directory: %w", err)
	}

	return ValidateRequirementsAt(cwd, cfg)
}

// ValidateRequirementsAt checks V1 release requirements at an explicit
// repository root without reading process cwd.
func ValidateRequirementsAt(repositoryRoot string, cfg *releaseconfig.V1ReleaseConfig) error {
	return legacyrequirements.Validate(repositoryRoot, cfg)
}

// ValidateRequirementsForContext checks executor requirements relative to the
// release unit root, not the caller's current working directory.
func ValidateRequirementsForContext(ctx *ReleaseExecutionContext) error {
	if ctx == nil {
		return fmt.Errorf("release execution context is missing")
	}

	log.PluginV(
		log.Config,
		"Validating release requirements for %s at the selected unit root",
		log.ColorText(log.ColorCyan, ctx.Executor),
	)

	return validateRequirementsForExecutor(ctx.Executor, ctx.UnitRoot, !ctx.DryRun)
}

func validateRequirementsForContextInspection(ctx *ReleaseExecutionContext) error {
	if ctx == nil {
		return fmt.Errorf("release execution context is missing")
	}
	return validateRequirementsForExecutor(ctx.Executor, ctx.UnitRoot, false)
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
	identity, err := releasetool.ParseIdentity(executor)
	if err != nil {
		return nil, fmt.Errorf("unknown release system: %s", executor)
	}
	return releasetool.ConfigCandidates(identity)
}

func joinQuotedFiles(files []string) string {
	quoted := make([]string, len(files))
	for i, file := range files {
		quoted[i] = fmt.Sprintf("%q", file)
	}

	return strings.Join(quoted, ", ")
}
