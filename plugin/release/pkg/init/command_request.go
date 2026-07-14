package init

import (
	"path/filepath"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// initCommandRequest and unitAddCommandRequest deliberately contain no raw
// flag map. Flag presence needed for compatibility is captured explicitly.
type initCommandRequest struct {
	Unit  v2UnitRequest
	Force bool
}

type unitAddCommandRequest struct {
	Unit         v2UnitRequest
	ForcePresent bool
}

type v2UnitRequest struct { //nolint:govet // Command fields follow their user-facing order.
	UnitID             string
	DisplayName        string
	Version            string
	Executor           config.ExecutorType
	Delivery           config.DeliveryType
	Workflow           string
	TagPrefix          string
	WorkingDirectory   string
	Paths              string
	Kind               string
	Plugin             config.V2Plugin
	LegacyFlagsPresent bool
	PluginFlagsPresent bool
}

func parseInitCommandRequest(flags map[string]any) initCommandRequest {
	return initCommandRequest{
		Unit:  parseV2UnitRequest(flags),
		Force: getFlagBool(flags, "force"),
	}
}

func parseUnitAddCommandRequest(flags map[string]any) unitAddCommandRequest {
	return unitAddCommandRequest{
		Unit:         parseV2UnitRequest(flags),
		ForcePresent: hasAnyFlag(flags, "force"),
	}
}

func parseV2UnitRequest(flags map[string]any) v2UnitRequest {
	return v2UnitRequest{
		UnitID:           getFlagString(flags, "unit"),
		DisplayName:      getFlagString(flags, "display-name"),
		Version:          getFlagString(flags, "version"),
		Executor:         config.ExecutorType(getFlagString(flags, "executor")),
		Delivery:         config.DeliveryType(getFlagString(flags, "delivery")),
		Workflow:         getFlagString(flags, "workflow"),
		TagPrefix:        getFlagString(flags, "tag-prefix"),
		WorkingDirectory: getFlagString(flags, "working-directory"),
		Paths:            getFlagString(flags, "paths"),
		Kind:             getFlagString(flags, "kind"),
		Plugin: config.V2Plugin{
			Name:        getFlagString(flags, "plugin-name"),
			Manifest:    filepath.ToSlash(getFlagString(flags, "plugin-manifest")),
			AssetPrefix: getFlagString(flags, "plugin-asset-prefix"),
			BinaryName:  getFlagString(flags, "plugin-binary-name"),
		},
		LegacyFlagsPresent: hasAnyFlag(flags, "project-type", "release-system", "metadata"),
		PluginFlagsPresent: hasAnyFlag(flags, pluginInitFlagNames...),
	}
}

func getFlagString(flags map[string]any, key string) string {
	if val, ok := flags[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getFlagBool(flags map[string]any, key string) bool {
	if val, ok := flags[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

func hasAnyFlag(flags map[string]any, names ...string) bool {
	for _, name := range names {
		if _, ok := flags[name]; ok {
			return true
		}
	}
	return false
}
