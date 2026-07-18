package init

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

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

type constructedV2Unit struct {
	Config v2InitConfig
	Unit   config.V2Unit
	State  config.V2UnitState
}

func constructV2Unit(request v2UnitRequest) (constructedV2Unit, error) {
	initConfig, err := normalizeV2UnitRequest(request)
	if err != nil {
		return constructedV2Unit{}, err
	}

	switch initConfig.Kind {
	case defaultKind:
		return constructReleaseUnit(initConfig), nil
	case pluginKind:
		return constructPluginUnit(initConfig), nil
	default:
		return constructedV2Unit{}, fmt.Errorf("invalid kind: %s (must be: release or plugin)", initConfig.Kind)
	}
}

func normalizeV2UnitRequest(request v2UnitRequest) (v2InitConfig, error) {
	if request.LegacyFlagsPresent {
		return v2InitConfig{}, fmt.Errorf("release init is V2-only; use --executor and --delivery instead of legacy project-type/release-system/metadata flags")
	}

	unitID := defaultString(request.UnitID, defaultUnitID)
	displayName := defaultString(request.DisplayName, unitID)
	version := defaultString(request.Version, defaultInitialVersion)
	tagPrefix := defaultString(request.TagPrefix, defaultTagPrefix)
	workingDirectory := defaultString(request.WorkingDirectory, defaultWorkingDirectory)
	paths, err := parsePaths(defaultString(request.Paths, defaultPaths))
	if err != nil {
		return v2InitConfig{}, err
	}
	kind := defaultString(request.Kind, defaultKind)
	if kind == defaultKind && request.PluginFlagsPresent {
		return v2InitConfig{}, fmt.Errorf("plugin flags require --kind plugin")
	}
	if kind == pluginKind {
		required := map[string]string{
			"plugin-name":         request.Plugin.Name,
			"plugin-manifest":     request.Plugin.Manifest,
			"plugin-asset-prefix": request.Plugin.AssetPrefix,
			"plugin-binary-name":  request.Plugin.BinaryName,
		}
		for _, flagName := range pluginInitFlagNames {
			if strings.TrimSpace(required[flagName]) == "" {
				return v2InitConfig{}, fmt.Errorf("--kind plugin requires --%s", flagName)
			}
		}
	}
	if kind != defaultKind && kind != pluginKind {
		return v2InitConfig{}, fmt.Errorf("invalid kind: %s (must be: release or plugin)", kind)
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
	if request.Executor == "" {
		return v2InitConfig{}, fmt.Errorf("missing required flag: --executor (goreleaser|jreleaser|release-it)")
	}
	if !request.Executor.IsValid() {
		return v2InitConfig{}, fmt.Errorf("invalid executor: %s (must be: goreleaser, jreleaser, or release-it)", request.Executor)
	}
	if request.Delivery == "" {
		return v2InitConfig{}, fmt.Errorf("missing required flag: --delivery (github-actions)")
	}
	if !request.Delivery.IsValid() {
		return v2InitConfig{}, fmt.Errorf("invalid delivery: %s (must be: github-actions)", request.Delivery)
	}
	if request.Delivery == config.DeliveryLocal {
		return v2InitConfig{}, fmt.Errorf("unsupported delivery: local (V2 releases support github-actions only)")
	}
	if request.Delivery != config.DeliveryGitHubActions {
		return v2InitConfig{}, fmt.Errorf("unsupported delivery: %s (V2 releases support github-actions only)", request.Delivery)
	}
	workflow := request.Workflow
	if strings.TrimSpace(workflow) == "" {
		return v2InitConfig{}, fmt.Errorf("github-actions delivery requires --workflow")
	}
	return v2InitConfig{
		UnitID:           unitID,
		DisplayName:      displayName,
		Version:          version,
		Executor:         request.Executor,
		Delivery:         request.Delivery,
		Workflow:         workflow,
		TagPrefix:        tagPrefix,
		WorkingDirectory: workingDirectory,
		Paths:            paths,
		Kind:             kind,
		Plugin:           request.Plugin,
	}, nil
}

func constructReleaseUnit(initConfig v2InitConfig) constructedV2Unit {
	return constructedV2Unit{
		Config: initConfig,
		Unit:   commonV2Unit(initConfig),
		State:  config.V2UnitState{Version: initConfig.Version},
	}
}

func constructPluginUnit(initConfig v2InitConfig) constructedV2Unit {
	unit := commonV2Unit(initConfig)
	unit.Kind = config.UnitKindPlugin
	pluginConfig := initConfig.Plugin
	unit.Plugin = &pluginConfig
	return constructedV2Unit{
		Config: initConfig,
		Unit:   unit,
		State:  config.V2UnitState{Version: initConfig.Version},
	}
}

func commonV2Unit(initConfig v2InitConfig) config.V2Unit {
	return config.V2Unit{
		ID:               initConfig.UnitID,
		DisplayName:      initConfig.DisplayName,
		Paths:            append([]string(nil), initConfig.Paths...),
		WorkingDirectory: initConfig.WorkingDirectory,
		TagPrefix:        initConfig.TagPrefix,
		Executor: config.V2Executor{
			Type:     initConfig.Executor,
			Delivery: initConfig.Delivery,
			Workflow: initConfig.Workflow,
		},
	}
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
