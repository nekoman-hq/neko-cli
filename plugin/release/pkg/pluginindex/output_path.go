package pluginindex

import (
	"fmt"
	"path/filepath"
	"strings"
)

type pluginIndexOutputTarget struct {
	ConfiguredPath string
	AbsolutePath   string
	External       bool
}

func resolvePluginIndexOutputTarget(repositoryRoot, configuredPath string) (pluginIndexOutputTarget, error) {
	configured := strings.TrimSpace(configuredPath)
	if configured == "" {
		return pluginIndexOutputTarget{}, fmt.Errorf("plugin index output path is required")
	}
	absoluteRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return pluginIndexOutputTarget{}, fmt.Errorf("resolve plugin index repository root: %w", err)
	}
	if filepath.IsAbs(configured) {
		absoluteTarget, err := filepath.Abs(configured)
		if err != nil {
			return pluginIndexOutputTarget{}, fmt.Errorf("resolve plugin index output %q: %w", configuredPath, err)
		}
		return pluginIndexOutputTarget{
			ConfiguredPath: configuredPath,
			AbsolutePath:   filepath.Clean(absoluteTarget),
			External:       true,
		}, nil
	}
	clean := filepath.Clean(filepath.FromSlash(configured))
	return pluginIndexOutputTarget{
		ConfiguredPath: configuredPath,
		AbsolutePath:   filepath.Join(absoluteRoot, clean),
		External:       false,
	}, nil
}
