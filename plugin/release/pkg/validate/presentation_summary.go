package validate

import (
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const (
	validationPresentationTitle = "Release Configuration Validation"
	validationUnitRoleKey       = "unit_role"
	validationVersionRoleKey    = "version_role"
	validationKindRoleKey       = "kind_role"
)

func validationSummaryPresentation(result validationQueryResult) *presentation.Properties {
	return &presentation.Properties{
		Title:      validationPresentationTitle,
		Properties: validationSummaryProperties(result),
	}
}

func validationSummaryProperties(result validationQueryResult) []presentation.Property {
	properties := []presentation.Property{
		{Label: "Status", Value: "✓ Valid", Role: presentation.StyleSuccess, Emphasized: true},
	}
	if result.SourceFormat == config.SourceFormatV1 {
		properties = append(properties,
			presentation.Property{Label: "Source", Value: "Legacy V1 config"},
			presentation.Property{Label: "Schema", Value: "v1"},
			presentation.Property{Label: "Configuration", Value: ".release.neko.json"},
		)
	} else {
		properties = append(properties,
			presentation.Property{Label: "Source", Value: "V2 config and state"},
			presentation.Property{Label: "Schema", Value: "v2"},
			presentation.Property{Label: "Configuration", Value: ".neko/release.config.json"},
			presentation.Property{Label: "State", Value: ".neko/release.state.json"},
		)
	}
	if result.SelectedUnit != "" {
		properties = append(properties, presentation.Property{Label: "Selected unit", Value: result.SelectedUnit})
	}
	if result.SourceFormat == config.SourceFormatV2 {
		properties = append(properties, presentation.Property{Label: "Configured units", Value: result.ConfiguredUnitCount})
	}
	return properties
}
