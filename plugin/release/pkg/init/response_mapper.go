package init

import (
	"fmt"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

func mapInitializeV2Response(result initializeV2Result, failure *commandFailure, timestamp time.Time) *plugin.Response {
	if failure != nil {
		return mapCommandFailure(failure, timestamp)
	}
	unit := result.Unit
	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "init",
			Timestamp: timestamp,
		},
		Data: map[string]any{
			"config_file":       config.V2ConfigPath("."),
			"state_file":        config.V2StatePath("."),
			"schema":            "v2",
			"unit":              unit.UnitID,
			"display_name":      unit.DisplayName,
			"version":           unit.Version,
			"executor":          string(unit.Executor),
			"delivery":          string(unit.Delivery),
			"workflow":          unit.Workflow,
			"tag_prefix":        unit.TagPrefix,
			"working_directory": unit.WorkingDirectory,
			"paths":             unit.Paths,
			"kind":              unit.Kind,
			"plugin":            pluginResponseData(unit),
			"next_steps":        buildV2NextSteps(unit),
		},
		RendererHint: "text",
	}
}

func mapAddV2UnitResponse(result addV2UnitResult, failure *commandFailure, timestamp time.Time) *plugin.Response {
	if failure != nil {
		// The legacy init command value for unit-add errors is intentionally
		// retained as a characterized compatibility contract.
		return mapCommandFailure(failure, timestamp)
	}
	unit := result.Unit
	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "unit-add",
			Timestamp: timestamp,
		},
		Data: map[string]any{
			"status":      "unit appended",
			"config_file": config.V2ConfigPath("."),
			"state_file":  config.V2StatePath("."),
			"unit":        unit.UnitID,
			"version":     unit.Version,
			"kind":        unit.Kind,
			"plugin":      pluginResponseData(unit),
		},
		RendererHint: "table",
	}
}

func mapCommandFailure(failure *commandFailure, timestamp time.Time) *plugin.Response {
	responseError := &plugin.ResponseError{
		Code:    failure.code,
		Message: failure.message,
	}
	if len(failure.details) > 0 {
		responseError.Details = failure.details
	}
	return &plugin.Response{
		Status: "error",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "init",
			Timestamp: timestamp,
		},
		Error: responseError,
	}
}

func pluginResponseData(initConfig v2InitConfig) map[string]any {
	if initConfig.Kind != pluginKind {
		return nil
	}
	return map[string]any{
		"name":         initConfig.Plugin.Name,
		"manifest":     initConfig.Plugin.Manifest,
		"asset_prefix": initConfig.Plugin.AssetPrefix,
		"binary_name":  initConfig.Plugin.BinaryName,
	}
}

func buildV2NextSteps(initConfig v2InitConfig) []string {
	steps := []string{
		"Use 'neko release validate --show' to inspect the V2 configuration",
	}
	if initConfig.Kind == pluginKind {
		steps = append(steps, "Use 'neko release plugin-index --check' after adding plugin units to verify registry metadata")
	}
	if initConfig.Delivery == config.DeliveryGitHubActions {
		steps = append(steps, fmt.Sprintf("Ensure %s builds and publishes from the dispatched tag", initConfig.Workflow))
	}
	return steps
}
