//nolint:staticcheck // V1 validation intentionally preserves the deprecated V1 requirements contract.
package validate

import (
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/legacyrequirements"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type legacyReleaseRequirementsValidator struct {
	repositoryRoot string
}

func (validator legacyReleaseRequirementsValidator) Validate(cfg *config.V1ReleaseConfig) error {
	return legacyrequirements.Validate(validator.repositoryRoot, cfg)
}
