// Package init includes the init handler for plugin-based execution
package init

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      03.02.2026
*/

import (
	"fmt"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/git"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

// HandleInit handles the init command in plugin mode
// It accepts configuration via flags instead of interactive prompts
func HandleInit(req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Init, "Starting release initialization")

	// Check for force flag to overwrite existing config
	force := getFlagBool(req.Flags, "force")

	// Check if config already exists
	if config.Exists() && !force {
		log.PluginV(log.Init, "Config file already exists, force flag not set")
		return &plugin.Response{
			Status: "error",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   "init",
				Timestamp: time.Now(),
			},
			Error: &plugin.ResponseError{
				Code:    "CONFIG_EXISTS",
				Message: fmt.Sprintf("%s already exists. Use --force to overwrite.", metadata.ConfigFileName),
			},
		}, nil
	}

	// Build config from flags
	cfg, err := buildConfigFromFlags(req.Flags)
	if err != nil {
		return &plugin.Response{
			Status: "error",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   "init",
				Timestamp: time.Now(),
			},
			Error: &plugin.ResponseError{
				Code:    "INVALID_FLAGS",
				Message: err.Error(),
				Details: map[string]any{
					"required_flags": []string{"project-type", "release-system"},
					"optional_flags": []string{"metadata", "force"},
				},
			},
		}, nil
	}

	// Try to get repo info from git
	repoInfo, _ := git.Current()
	if repoInfo != nil {
		cfg.ProjectOwner = repoInfo.Owner
		cfg.ProjectName = repoInfo.Repo
		log.PluginV(log.Init, "Detected repository: %s/%s", repoInfo.Owner, repoInfo.Repo)
	}

	// Validate the config
	if err = config.Validate(&cfg); err != nil {
		return &plugin.Response{
			Status: "error",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   "init",
				Timestamp: time.Now(),
			},
			Error: &plugin.ResponseError{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		}, nil
	}

	// Save the config
	if err = config.SaveConfig(cfg); err != nil {
		return &plugin.Response{
			Status: "error",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   "init",
				Timestamp: time.Now(),
			},
			Error: &plugin.ResponseError{
				Code:    "SAVE_ERROR",
				Message: fmt.Sprintf("Failed to save configuration: %v", err),
			},
		}, nil
	}

	log.PluginPrint(log.Init, "Configuration saved to %s", metadata.ConfigFileName)

	// Initialize the release system
	releaser, err := release.Get(string(cfg.ReleaseSystem))
	if err != nil {
		return &plugin.Response{
			Status: "error",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   "init",
				Timestamp: time.Now(),
			},
			Error: &plugin.ResponseError{
				Code:    "RELEASE_SYSTEM_ERROR",
				Message: fmt.Sprintf("Release system not found: %v", err),
			},
		}, nil
	}

	if err := releaser.Init(&cfg); err != nil {
		log.PluginV(log.Init, "Release system initialization failed: %v", err)
		// Don't fail completely, config is saved
	} else {
		log.PluginPrint(log.Init, "Release system %s initialized", cfg.ReleaseSystem)
	}

	log.PluginPrint(log.Init, "Initialization completed successfully")

	// Build next steps based on release system
	nextSteps := buildNextSteps(cfg)

	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "init",
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"config_file":    metadata.ConfigFileName,
			"project_name":   cfg.ProjectName,
			"project_owner":  cfg.ProjectOwner,
			"project_type":   string(cfg.ProjectType),
			"release_system": string(cfg.ReleaseSystem),
			"metadata":       cfg.Version,
			"next_steps":     nextSteps,
		},
		RendererHint: "text",
	}, nil
}

// GetAvailableOptions returns the available options for init configuration
// This can be used by the CLI to show help or provide autocomplete
func GetAvailableOptions() (*plugin.Response, error) {
	// Build items as a table-friendly format
	items := []map[string]any{
		{
			"option":      "project-type",
			"values":      "frontend, backend, other",
			"required":    true,
			"description": "Type of project being released",
		},
		{
			"option":      "release-system",
			"values":      "release-it, jreleaser, goreleaser",
			"required":    true,
			"description": "Release tool to use",
		},
		{
			"option":      "metadata",
			"values":      "semver (e.g. 0.1.0)",
			"required":    false,
			"description": "Initial metadata (default: 0.1.0)",
		},
		{
			"option":      "force",
			"values":      "true, false",
			"required":    false,
			"description": "Overwrite existing config",
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
			"recommendations": map[string]string{
				"frontend": string(config.ReleaseTypeReleaseIt),
				"backend":  string(config.ReleaseTypeJReleaser),
				"other":    string(config.ReleaseTypeGoReleaser),
			},
		},
		RendererHint: "table",
	}, nil
}

func buildConfigFromFlags(flags map[string]any) (config.NekoConfig, error) {
	cfg := config.NekoConfig{}

	// Get project type (required)
	projectType := getFlagString(flags, "project-type")
	if projectType == "" {
		return cfg, fmt.Errorf("missing required flag: --project-type (frontend|backend|other)")
	}
	cfg.ProjectType = config.ProjectType(projectType)
	if !cfg.ProjectType.IsValid() {
		return cfg, fmt.Errorf("invalid project type: %s (must be: frontend, backend, or other)", projectType)
	}

	// Get release system (required)
	releaseSystem := getFlagString(flags, "release-system")
	if releaseSystem == "" {
		return cfg, fmt.Errorf("missing required flag: --release-system (release-it|jreleaser|goreleaser)")
	}
	cfg.ReleaseSystem = config.ReleaseSystem(releaseSystem)
	if !cfg.ReleaseSystem.IsValid() {
		return cfg, fmt.Errorf("invalid release system: %s (must be: release-it, jreleaser, or goreleaser)", releaseSystem)
	}

	// Get metadata (optional, defaults to 0.1.0)
	versionStr := getFlagString(flags, "metadata")
	if versionStr == "" {
		versionStr = "0.1.0"
	}
	cfg.Version = versionStr

	return cfg, nil
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

func buildNextSteps(cfg config.NekoConfig) []string {
	steps := []string{
		"Use 'neko release' to create a release",
	}

	switch cfg.ReleaseSystem {
	case config.ReleaseTypeReleaseIt:
		steps = append(steps,
			"Neko will manage metadata in: package.json, .release-it.json",
		)
	case config.ReleaseTypeJReleaser:
		steps = append(steps,
			"Neko will manage metadata in: jreleaser.yml, pom.xml / build.gradle",
		)
	case config.ReleaseTypeGoReleaser:
		steps = append(steps,
			"Neko will manage metadata in: .goreleaser.yml, Git tags",
		)
	}

	steps = append(steps,
		fmt.Sprintf("The metadata in %s is the single source of truth", metadata.ConfigFileName),
	)

	return steps
}
