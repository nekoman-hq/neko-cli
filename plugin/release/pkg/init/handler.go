// Package init includes the init handler for plugin-based execution.
package init

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      03.02.2026
*/

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

const (
	defaultUnitID           = "cli"
	defaultInitialVersion   = "0.1.0"
	defaultTagPrefix        = "v"
	defaultWorkingDirectory = "."
	defaultPaths            = "**"
	defaultKind             = "release"
	pluginKind              = "plugin"
	legacyV1ConfigFileName  = ".release.neko.json"
)

var pluginInitFlagNames = []string{
	"plugin-name",
	"plugin-manifest",
	"plugin-asset-prefix",
	"plugin-binary-name",
}

// HandleInit handles the init command in plugin mode
// It accepts configuration via flags instead of interactive prompts
func HandleInit(req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Init, "Starting release initialization")

	force := getFlagBool(req.Flags, "force")

	if err := checkExistingConfig(force); err != nil {
		log.PluginV(log.Init, "Existing release configuration prevents init: %s", err.message)
		return initErrorResponse(err.code, err.message, nil), nil
	}

	initConfig, err := buildV2InitConfigFromFlags(req.Flags)
	if err != nil {
		return initErrorResponse("INVALID_FLAGS", err.Error(), map[string]any{
			"required_flags": []string{"executor", "delivery"},
			"optional_flags": []string{
				"unit",
				"display-name",
				"version",
				"workflow",
				"tag-prefix",
				"working-directory",
				"paths",
				"kind",
				"plugin-name",
				"plugin-manifest",
				"plugin-asset-prefix",
				"plugin-binary-name",
				"force",
			},
		}), nil
	}

	cfg, state := buildV2Files(initConfig)
	if err = config.ValidateV2(".", &cfg, &state); err != nil {
		return initErrorResponse("VALIDATION_ERROR", err.Error(), nil), nil
	}

	configJSON, err := config.CanonicalV2Config(cfg)
	if err != nil {
		return initErrorResponse("VALIDATION_ERROR", err.Error(), nil), nil
	}
	stateJSON, err := config.CanonicalV2State(state)
	if err != nil {
		return initErrorResponse("VALIDATION_ERROR", err.Error(), nil), nil
	}
	if err = writeV2Files(configJSON, stateJSON); err != nil {
		return initErrorResponse("SAVE_ERROR", fmt.Sprintf("Failed to save V2 release configuration: %v", err), nil), nil
	}

	log.PluginPrint(log.Init, "Configuration saved to %s", config.V2ConfigPath("."))
	log.PluginPrint(log.Init, "State saved to %s", config.V2StatePath("."))
	log.PluginPrint(log.Init, "Initialization completed successfully")

	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "init",
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"config_file":       config.V2ConfigPath("."),
			"state_file":        config.V2StatePath("."),
			"schema":            "v2",
			"unit":              initConfig.UnitID,
			"display_name":      initConfig.DisplayName,
			"version":           initConfig.Version,
			"executor":          string(initConfig.Executor),
			"delivery":          string(initConfig.Delivery),
			"workflow":          initConfig.Workflow,
			"tag_prefix":        initConfig.TagPrefix,
			"working_directory": initConfig.WorkingDirectory,
			"paths":             initConfig.Paths,
			"kind":              initConfig.Kind,
			"plugin":            pluginResponseData(initConfig),
			"next_steps":        buildV2NextSteps(initConfig),
		},
		RendererHint: "text",
	}, nil
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

// GetAvailableOptions returns the available options for init configuration
// This can be used by the CLI to show help or provide autocomplete
func GetAvailableOptions() (*plugin.Response, error) {
	// Build items as a table-friendly format
	items := []map[string]any{
		{
			"option":      "unit",
			"values":      "cli, api, plugin-release, ...",
			"required":    false,
			"description": "Release unit id",
		},
		{
			"option":      "display-name",
			"values":      "string",
			"required":    false,
			"description": "Release unit display name",
		},
		{
			"option":      "version",
			"values":      "semver, default 0.1.0",
			"required":    false,
			"description": "Initial version",
		},
		{
			"option":      "executor",
			"values":      "goreleaser, jreleaser, release-it",
			"required":    true,
			"description": "Release executor",
		},
		{
			"option":      "delivery",
			"values":      "local, github-actions",
			"required":    true,
			"description": "Release delivery mode",
		},
		{
			"option":      "workflow",
			"values":      ".github/workflows/*.yml",
			"required":    "conditional",
			"description": "GitHub Actions workflow path",
		},
		{
			"option":      "tag-prefix",
			"values":      "v",
			"required":    false,
			"description": "Release tag prefix",
		},
		{
			"option":      "working-directory",
			"values":      ".",
			"required":    false,
			"description": "Unit working directory",
		},
		{
			"option":      "paths",
			"values":      "comma-separated globs",
			"required":    false,
			"description": "Unit path scope",
		},
		{
			"option":      "kind",
			"values":      "release, plugin",
			"required":    false,
			"description": "Release unit kind",
		},
		{
			"option":      "plugin-name",
			"values":      "release, ui, ...",
			"required":    "when kind=plugin",
			"description": "Public plugin registry name",
		},
		{
			"option":      "plugin-manifest",
			"values":      "plugin/<name>/manifest.json",
			"required":    "when kind=plugin",
			"description": "Repository-root-relative plugin manifest path",
		},
		{
			"option":      "plugin-asset-prefix",
			"values":      "plugin-<name>",
			"required":    "when kind=plugin",
			"description": "Plugin release asset prefix, must match unit id",
		},
		{
			"option":      "plugin-binary-name",
			"values":      "plugin-<name>",
			"required":    "when kind=plugin",
			"description": "Plugin executable name in release archives",
		},
		{
			"option":      "force",
			"values":      "true, false",
			"required":    false,
			"description": "Overwrite existing V2 config/state",
		},
	}

	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "init-options",
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"items": items,
		},
		RendererHint: "table",
	}, nil
}

type initConfigError struct {
	code    string
	message string
}

func (err *initConfigError) Error() string {
	return err.message
}

type v2InitConfig struct { //nolint:govet // Init config keeps CLI/domain option order for readability.
	UnitID           string
	DisplayName      string
	Version          string
	Executor         config.ExecutorType
	Delivery         config.DeliveryType
	Workflow         string
	TagPrefix        string
	WorkingDirectory string
	Paths            []string
	Kind             string
	Plugin           config.V2Plugin
}

func checkExistingConfig(force bool) *initConfigError {
	hasV1 := fileExists(legacyV1ConfigFileName)
	hasV2Config := config.V2ConfigExists(".")
	hasV2State := config.V2StateExists(".")
	hasAnyV2 := hasV2Config || hasV2State

	if hasV1 && hasAnyV2 {
		return &initConfigError{
			code:    "CONFIG_CONFLICT",
			message: "release configuration conflict: .release.neko.json and V2 files both exist. Resolve the conflict explicitly before running init.",
		}
	}
	if hasV1 {
		return &initConfigError{
			code:    "V1_CONFIG_EXISTS",
			message: ".release.neko.json already exists. Run 'neko release migrate' to convert it to V2; init will not overwrite V1 configs.",
		}
	}
	if hasAnyV2 && !force {
		return &initConfigError{
			code:    "CONFIG_EXISTS",
			message: ".neko/release.config.json or .neko/release.state.json already exists. Use --force to recreate both V2 files.",
		}
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func buildV2InitConfigFromFlags(flags map[string]any) (v2InitConfig, error) {
	if hasAnyFlag(flags, "project-type", "release-system", "metadata") {
		return v2InitConfig{}, fmt.Errorf("release init is V2-only; use --executor and --delivery instead of legacy project-type/release-system/metadata flags")
	}

	unitID := defaultString(getFlagString(flags, "unit"), defaultUnitID)
	displayName := defaultString(getFlagString(flags, "display-name"), unitID)
	version := defaultString(getFlagString(flags, "version"), defaultInitialVersion)
	executor := config.ExecutorType(getFlagString(flags, "executor"))
	delivery := config.DeliveryType(getFlagString(flags, "delivery"))
	workflow := getFlagString(flags, "workflow")
	tagPrefix := defaultString(getFlagString(flags, "tag-prefix"), defaultTagPrefix)
	workingDirectory := defaultString(getFlagString(flags, "working-directory"), defaultWorkingDirectory)
	paths, err := parsePaths(defaultString(getFlagString(flags, "paths"), defaultPaths))
	if err != nil {
		return v2InitConfig{}, err
	}
	kind := defaultString(getFlagString(flags, "kind"), defaultKind)
	pluginConfig, err := buildPluginConfigFromFlags(kind, flags)
	if err != nil {
		return v2InitConfig{}, err
	}

	if strings.TrimSpace(displayName) == "" {
		return v2InitConfig{}, fmt.Errorf("display-name must not be empty")
	}
	if version != strings.TrimSpace(version) || strings.HasPrefix(version, "v") {
		return v2InitConfig{}, fmt.Errorf("version %q must be SemVer without leading v", version)
	}
	if _, err := semver.NewVersion(version); err != nil {
		return v2InitConfig{}, fmt.Errorf("version %q must be valid SemVer: %w", version, err)
	}
	if executor == "" {
		return v2InitConfig{}, fmt.Errorf("missing required flag: --executor (goreleaser|jreleaser|release-it)")
	}
	if !executor.IsValid() {
		return v2InitConfig{}, fmt.Errorf("invalid executor: %s (must be: goreleaser, jreleaser, or release-it)", executor)
	}
	if delivery == "" {
		return v2InitConfig{}, fmt.Errorf("missing required flag: --delivery (local|github-actions)")
	}
	if !delivery.IsValid() {
		return v2InitConfig{}, fmt.Errorf("invalid delivery: %s (must be: local or github-actions)", delivery)
	}
	if delivery == config.DeliveryLocal {
		workflow = ""
	}

	return v2InitConfig{
		UnitID:           unitID,
		DisplayName:      displayName,
		Version:          version,
		Executor:         executor,
		Delivery:         delivery,
		Workflow:         workflow,
		TagPrefix:        tagPrefix,
		WorkingDirectory: workingDirectory,
		Paths:            paths,
		Kind:             kind,
		Plugin:           pluginConfig,
	}, nil
}

func buildPluginConfigFromFlags(kind string, flags map[string]any) (config.V2Plugin, error) {
	switch kind {
	case defaultKind:
		if hasAnyFlag(flags, pluginInitFlagNames...) {
			return config.V2Plugin{}, fmt.Errorf("plugin flags require --kind plugin")
		}
		return config.V2Plugin{}, nil
	case pluginKind:
		pluginConfig := config.V2Plugin{
			Name:        getFlagString(flags, "plugin-name"),
			Manifest:    filepath.ToSlash(getFlagString(flags, "plugin-manifest")),
			AssetPrefix: getFlagString(flags, "plugin-asset-prefix"),
			BinaryName:  getFlagString(flags, "plugin-binary-name"),
		}
		required := map[string]string{
			"plugin-name":         pluginConfig.Name,
			"plugin-manifest":     pluginConfig.Manifest,
			"plugin-asset-prefix": pluginConfig.AssetPrefix,
			"plugin-binary-name":  pluginConfig.BinaryName,
		}
		for _, flagName := range pluginInitFlagNames {
			if strings.TrimSpace(required[flagName]) == "" {
				return config.V2Plugin{}, fmt.Errorf("--kind plugin requires --%s", flagName)
			}
		}
		return pluginConfig, nil
	default:
		return config.V2Plugin{}, fmt.Errorf("invalid kind: %s (must be: release or plugin)", kind)
	}
}

func buildV2Files(initConfig v2InitConfig) (config.V2ReleaseConfig, config.V2ReleaseState) {
	unit := config.V2Unit{
		ID:               initConfig.UnitID,
		DisplayName:      initConfig.DisplayName,
		Paths:            initConfig.Paths,
		WorkingDirectory: initConfig.WorkingDirectory,
		TagPrefix:        initConfig.TagPrefix,
		Executor: config.V2Executor{
			Type:     initConfig.Executor,
			Delivery: initConfig.Delivery,
			Workflow: initConfig.Workflow,
		},
	}
	if initConfig.Kind == pluginKind {
		unit.Kind = config.UnitKindPlugin
		unit.Plugin = &initConfig.Plugin
	}

	cfg := config.V2ReleaseConfig{
		SchemaVersion: 2,
		Units:         []config.V2Unit{unit},
	}
	state := config.V2ReleaseState{
		SchemaVersion: 2,
		Units: map[string]config.V2UnitState{
			initConfig.UnitID: {Version: initConfig.Version},
		},
	}
	return cfg, state
}

func writeV2Files(configJSON, stateJSON []byte) error {
	if err := os.MkdirAll(config.V2Directory, 0755); err != nil {
		return fmt.Errorf("create %s directory: %w", config.V2Directory, err)
	}
	configPath := config.V2ConfigPath(".")
	statePath := config.V2StatePath(".")
	if err := config.AtomicWriteFile(configPath, configJSON, 0644); err != nil {
		return err
	}
	if err := config.AtomicWriteFile(statePath, stateJSON, 0644); err != nil {
		return err
	}
	return nil
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

func parsePaths(value string) ([]string, error) {
	parts := strings.Split(value, ",")
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, fmt.Errorf("paths must not contain empty entries")
		}
		paths = append(paths, filepath.ToSlash(trimmed))
	}
	return paths, nil
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func hasAnyFlag(flags map[string]any, names ...string) bool {
	for _, name := range names {
		if _, ok := flags[name]; ok {
			return true
		}
	}
	return false
}

func initErrorResponse(code, message string, details map[string]any) *plugin.Response {
	responseError := &plugin.ResponseError{
		Code:    code,
		Message: message,
	}
	if len(details) > 0 {
		responseError.Details = details
	}
	return &plugin.Response{
		Status: "error",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "init",
			Timestamp: time.Now(),
		},
		Error: responseError,
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
	} else {
		steps = append(steps, "Add or verify the executor-specific local release configuration before running a real release")
	}
	return steps
}
