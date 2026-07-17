package pluginindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const (
	pluginIndexLegacyReleaseConfigPath = ".release.neko.json"
	pluginIndexLegacyReleaseBackupPath = ".release.neko.json.v1.bak"
)

type pluginIndexOutputTarget struct {
	ConfiguredPath string
	AbsolutePath   string
	External       bool
}

func resolvePluginIndexOutputTarget(repositoryRoot, configuredPath string, index *Index) (pluginIndexOutputTarget, error) {
	configured := strings.TrimSpace(configuredPath)
	if configured == "" {
		return pluginIndexOutputTarget{}, fmt.Errorf("plugin index output path is required")
	}
	if configured != configuredPath {
		return pluginIndexOutputTarget{}, fmt.Errorf("plugin index output path %q must not have leading or trailing whitespace", configuredPath)
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
		target := pluginIndexOutputTarget{
			ConfiguredPath: configuredPath,
			AbsolutePath:   filepath.Clean(absoluteTarget),
			External:       true,
		}
		if err := validatePluginIndexOutputTarget(absoluteRoot, target.AbsolutePath, true, index); err != nil {
			return pluginIndexOutputTarget{}, err
		}
		return target, nil
	}
	if strings.Contains(configured, `\`) {
		return pluginIndexOutputTarget{}, fmt.Errorf("plugin index output path %q must use forward slashes", configuredPath)
	}
	clean := filepath.Clean(filepath.FromSlash(configured))
	cleanDisplay := filepath.ToSlash(clean)
	if cleanDisplay == "." || cleanDisplay == ".." || strings.HasPrefix(cleanDisplay, "../") || cleanDisplay != configured {
		return pluginIndexOutputTarget{}, fmt.Errorf("plugin index output path %q must be a clean repository-root-relative path", configuredPath)
	}
	target := pluginIndexOutputTarget{
		ConfiguredPath: configuredPath,
		AbsolutePath:   filepath.Join(absoluteRoot, clean),
		External:       false,
	}
	if err := validatePluginIndexOutputTarget(absoluteRoot, target.AbsolutePath, false, index); err != nil {
		return pluginIndexOutputTarget{}, err
	}
	return target, nil
}

func validatePluginIndexOutputTarget(repositoryRoot, outputPath string, allowExternal bool, index *Index) error {
	relative, insideRepository := repositoryRelativePluginIndexPath(repositoryRoot, outputPath)
	if insideRepository {
		if err := rejectProtectedPluginIndexRepositoryPath(relative, index); err != nil {
			return err
		}
	}
	if !insideRepository && !allowExternal {
		return fmt.Errorf("plugin index output path %s resolves outside repository root", outputPath)
	}
	if err := validatePluginIndexOutputParent(repositoryRoot, outputPath, insideRepository); err != nil {
		return err
	}
	return rejectPluginIndexTargetSymlinkOrDirectory(outputPath)
}

func repositoryRelativePluginIndexPath(repositoryRoot, candidate string) (string, bool) {
	relative, err := filepath.Rel(repositoryRoot, candidate)
	if err != nil {
		return "", false
	}
	if relative == "." {
		return relative, true
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

func rejectProtectedPluginIndexRepositoryPath(relative string, index *Index) error {
	protected := map[string]string{
		filepath.ToSlash(filepath.Join(releaseconfig.V2Directory, releaseconfig.V2ConfigFileName)): "release configuration",
		filepath.ToSlash(filepath.Join(releaseconfig.V2Directory, releaseconfig.V2StateFileName)):  "release state",
		filepath.ToSlash(filepath.Join(releaseconfig.V2Directory, "release.pair-recovery.json")):   "release pair-recovery evidence",
		filepath.ToSlash(filepath.Join(releaseconfig.V2Directory, "release.migration.json")):       "release migration evidence",
		pluginIndexLegacyReleaseConfigPath: "legacy release configuration",
		pluginIndexLegacyReleaseBackupPath: "legacy release backup",
	}
	if index != nil {
		for _, entry := range index.Plugins {
			if entry.Manifest != "" {
				protected[entry.Manifest] = "plugin manifest"
			}
		}
	}
	if strings.HasPrefix(relative, ".git/") || relative == ".git" {
		return fmt.Errorf("plugin index output path %q targets internal Git state", relative)
	}
	if label, ok := protected[relative]; ok {
		return fmt.Errorf("plugin index output path %q targets protected %s", relative, label)
	}
	return nil
}

func rejectPluginIndexTargetSymlinkOrDirectory(outputPath string) error {
	info, err := os.Lstat(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect plugin index output %s: %w", outputPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("plugin index output path %s is a symlink", outputPath)
	}
	if info.IsDir() {
		return fmt.Errorf("plugin index output path %s is a directory", outputPath)
	}
	return nil
}

func validatePluginIndexOutputParent(repositoryRoot, outputPath string, insideRepository bool) error {
	parent, err := existingPluginIndexOutputParent(outputPath)
	if err != nil {
		return err
	}
	if !insideRepository {
		return nil
	}
	resolvedRoot, err := filepath.EvalSymlinks(repositoryRoot)
	if err != nil {
		return fmt.Errorf("resolve plugin index repository root %s: %w", repositoryRoot, err)
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return fmt.Errorf("resolve plugin index output parent %s: %w", parent, err)
	}
	if _, ok := repositoryRelativePluginIndexPath(resolvedRoot, resolvedParent); !ok {
		return fmt.Errorf("plugin index output parent %s resolves outside repository root", parent)
	}
	return nil
}

func existingPluginIndexOutputParent(outputPath string) (string, error) {
	parent := filepath.Dir(outputPath)
	for {
		info, err := os.Lstat(parent)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				resolvedParent, resolveErr := filepath.EvalSymlinks(parent)
				if resolveErr != nil {
					return "", fmt.Errorf("resolve plugin index output parent %s: %w", parent, resolveErr)
				}
				resolvedInfo, statErr := os.Stat(resolvedParent)
				if statErr != nil {
					return "", fmt.Errorf("inspect plugin index output parent %s: %w", parent, statErr)
				}
				if !resolvedInfo.IsDir() {
					return "", fmt.Errorf("plugin index output parent %s is not a directory", parent)
				}
				return parent, nil
			}
			if !info.IsDir() {
				return "", fmt.Errorf("plugin index output parent %s is not a directory", parent)
			}
			return parent, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect plugin index output parent %s: %w", parent, err)
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", fmt.Errorf("plugin index output parent %s does not exist", filepath.Dir(outputPath))
		}
		parent = next
	}
}
