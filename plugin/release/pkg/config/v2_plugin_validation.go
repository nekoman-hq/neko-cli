package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	pluginPublicNamePattern   = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	pluginArtifactNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

func validateV2UnitPluginMetadata(repositoryRoot string, unit V2Unit) error {
	if unit.Kind != "" && unit.Kind != UnitKindPlugin {
		return fmt.Errorf("v2 config unit %q has unknown kind %q", unit.ID, unit.Kind)
	}
	if unit.Kind == "" {
		if unit.Plugin != nil {
			return fmt.Errorf("v2 config unit %q plugin metadata requires kind %q", unit.ID, UnitKindPlugin)
		}
		return nil
	}
	if unit.Plugin == nil {
		return fmt.Errorf("v2 config unit %q kind %q requires plugin metadata", unit.ID, UnitKindPlugin)
	}
	if !strings.HasPrefix(unit.ID, "plugin-") {
		return fmt.Errorf("v2 config plugin unit %q id must start with plugin-", unit.ID)
	}
	expectedTagPrefix := unit.ID + "/v"
	if unit.TagPrefix != expectedTagPrefix {
		return fmt.Errorf("v2 config plugin unit %q tagPrefix must be %q", unit.ID, expectedTagPrefix)
	}
	if err := validatePluginName(unit.ID, unit.Plugin.Name); err != nil {
		return err
	}
	if err := validatePluginManifest(repositoryRoot, unit.ID, unit.Plugin.Manifest); err != nil {
		return err
	}
	if err := validatePluginArtifactName(unit.ID, "assetPrefix", unit.Plugin.AssetPrefix); err != nil {
		return err
	}
	if unit.Plugin.AssetPrefix != unit.ID {
		return fmt.Errorf("v2 config plugin unit %q assetPrefix must equal unit id %q", unit.ID, unit.ID)
	}
	if err := validatePluginArtifactName(unit.ID, "binaryName", unit.Plugin.BinaryName); err != nil {
		return err
	}
	return nil
}

func validatePluginName(unitID, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("v2 config plugin unit %q plugin.name is required", unitID)
	}
	if name != strings.TrimSpace(name) || !pluginPublicNamePattern.MatchString(name) {
		return fmt.Errorf("v2 config plugin unit %q plugin.name %q must match [a-z][a-z0-9-]*", unitID, name)
	}
	if strings.HasPrefix(name, "plugin-") {
		return fmt.Errorf("v2 config plugin unit %q plugin.name %q must not start with plugin-", unitID, name)
	}
	return nil
}

func validatePluginManifest(repositoryRoot, unitID, manifest string) error {
	if strings.TrimSpace(manifest) == "" {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest is required", unitID)
	}
	if manifest != strings.TrimSpace(manifest) {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest must not have leading or trailing whitespace", unitID)
	}
	if strings.Contains(manifest, `\`) {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q must use forward slashes", unitID, manifest)
	}
	if strings.HasPrefix(manifest, "/") || filepath.IsAbs(manifest) {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q must be repository-root-relative", unitID, manifest)
	}
	if strings.Contains(manifest, "://") || strings.ContainsAny(manifest, "?#@$`~{}[]!*") {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q must not contain URL, query, fragment, ref, or shell syntax", unitID, manifest)
	}
	clean := path.Clean(manifest)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != manifest {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q must be a clean repository-root-relative path", unitID, manifest)
	}
	if !strings.HasSuffix(manifest, "/manifest.json") && manifest != "manifest.json" {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q must end with manifest.json", unitID, manifest)
	}
	if repositoryRoot == "" {
		return nil
	}
	return validatePluginManifestAtRepositoryRoot(repositoryRoot, unitID, manifest)
}

func validatePluginManifestAtRepositoryRoot(repositoryRoot, unitID, manifest string) error {
	absoluteRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return fmt.Errorf("v2 config plugin unit %q repository root %q cannot be resolved: %w", unitID, repositoryRoot, err)
	}
	manifestPath := filepath.Join(absoluteRoot, filepath.FromSlash(manifest))
	info, err := os.Lstat(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q does not exist", unitID, manifest)
		}
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q cannot be inspected: %w", unitID, manifest, err)
	}
	if info.IsDir() {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q is a directory, expected a file", unitID, manifest)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return fmt.Errorf("v2 config plugin unit %q repository root %q cannot be resolved physically: %w", unitID, absoluteRoot, err)
	}
	resolvedManifest, err := filepath.EvalSymlinks(manifestPath)
	if err != nil {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q cannot be resolved physically: %w", unitID, manifest, err)
	}
	if !pathInside(resolvedRoot, resolvedManifest) {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q resolves outside repository root", unitID, manifest)
	}
	resolvedInfo, err := os.Stat(resolvedManifest)
	if err != nil {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q cannot be inspected after symlink resolution: %w", unitID, manifest, err)
	}
	if !resolvedInfo.Mode().IsRegular() {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q is not a regular file", unitID, manifest)
	}
	if filepath.Base(resolvedManifest) != "manifest.json" {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q must resolve to manifest.json", unitID, manifest)
	}
	return nil
}

func validatePluginArtifactName(unitID, field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("v2 config plugin unit %q plugin.%s is required", unitID, field)
	}
	if value != strings.TrimSpace(value) || !pluginArtifactNamePattern.MatchString(value) {
		return fmt.Errorf("v2 config plugin unit %q plugin.%s %q must match [a-z0-9][a-z0-9-]*", unitID, field, value)
	}
	if strings.Contains(value, ".") || strings.ContainsAny(value, `/\ `+"\t\r\n$`;&|<>") {
		return fmt.Errorf("v2 config plugin unit %q plugin.%s %q must be a conservative filename without slashes, extension, or shell syntax", unitID, field, value)
	}
	return nil
}
