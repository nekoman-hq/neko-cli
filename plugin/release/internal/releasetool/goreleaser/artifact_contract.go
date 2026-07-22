package goreleaser

import (
	"sort"
	"strings"
)

type ArtifactExpectation struct {
	UnitID         string
	BuildID        string
	Binary         string
	ChecksumPrefix string
	Plugin         bool
}

// PlatformFormats names the archive format expected for each supported
// installer platform.
type PlatformFormats struct {
	Darwin  string
	Linux   string
	Windows string
}

type FindingKind string

const (
	FindingConfigurationUnsupported FindingKind = "configuration-unsupported"
	FindingBuildIdentityMismatch    FindingKind = "build-identity-mismatch"
	FindingBinaryMismatch           FindingKind = "binary-mismatch"
	FindingPlatformMismatch         FindingKind = "platform-mismatch"
	FindingArchiveIdentityMismatch  FindingKind = "archive-identity-mismatch"
	FindingArchiveMismatch          FindingKind = "archive-mismatch"
	FindingChecksumMissing          FindingKind = "checksum-missing"
	FindingReleaseIdentityMismatch  FindingKind = "release-identity-mismatch"
)

// Finding is a neutral artifact-contract mismatch. Doctor owns the eventual
// severity, diagnostic code, prose, and remediation.
type Finding struct {
	Kind           FindingKind
	UnitID         string
	ExpectedID     string
	ExpectedBinary string
}

// VerifyArtifactContract compares focused GoReleaser facts with explicit unit
// expectations without reading files or producing Doctor diagnostics.
func VerifyArtifactContract(config Config, expectations []ArtifactExpectation) []Finding {
	if config.Version != 2 || config.ProjectName == "" {
		return []Finding{{Kind: FindingConfigurationUnsupported}}
	}
	findings := make([]Finding, 0)
	for _, expectation := range expectations {
		build, buildOK := BuildByID(config.Builds, expectation.BuildID)
		if !buildOK {
			findings = append(findings, Finding{Kind: FindingBuildIdentityMismatch, UnitID: expectation.UnitID, ExpectedID: expectation.BuildID})
			continue
		}
		if build.Binary != expectation.Binary || strings.TrimSpace(build.Main) == "" {
			findings = append(findings, Finding{Kind: FindingBinaryMismatch, UnitID: expectation.UnitID, ExpectedID: expectation.BuildID, ExpectedBinary: expectation.Binary})
		}
		if !containsAll(build.Goos, "darwin", "linux", "windows") {
			findings = append(findings, Finding{Kind: FindingPlatformMismatch, UnitID: expectation.UnitID, ExpectedID: expectation.BuildID})
		}
		archive, archiveOK := ArchiveByID(config.Archives, expectation.BuildID)
		if !archiveOK || !contains(archive.IDs, expectation.BuildID) {
			findings = append(findings, Finding{Kind: FindingArchiveIdentityMismatch, UnitID: expectation.UnitID, ExpectedID: expectation.BuildID})
			continue
		}
		if !contains(archive.Formats, "tar.gz") ||
			!strings.Contains(archive.NameTemplate, expectation.BuildID+"_") ||
			!strings.Contains(archive.NameTemplate, ".Os") ||
			!strings.Contains(archive.NameTemplate, ".Arch") {
			findings = append(findings, Finding{Kind: FindingArchiveMismatch, UnitID: expectation.UnitID, ExpectedID: expectation.BuildID})
		}
		if expectation.Plugin && (config.Checksum == nil ||
			!strings.Contains(config.Checksum.NameTemplate, expectation.ChecksumPrefix+"_") ||
			!strings.Contains(config.Checksum.NameTemplate, "checksums.txt")) {
			findings = append(findings, Finding{Kind: FindingChecksumMissing, UnitID: expectation.UnitID, ExpectedID: expectation.BuildID})
		}
		if !contains(config.Release.IDs, expectation.BuildID) {
			findings = append(findings, Finding{Kind: FindingReleaseIdentityMismatch, UnitID: expectation.UnitID, ExpectedID: expectation.BuildID})
		}
	}
	return findings
}

// CLIArchiveSupportsInstallation verifies the focused CLI archive shape used
// by the existing installer contract.
func CLIArchiveSupportsInstallation(config Config, archiveID string) bool {
	archive, ok := ArchiveByID(config.Archives, archiveID)
	if !ok || !contains(archive.Formats, "tar.gz") ||
		!strings.Contains(archive.NameTemplate, archiveID+"_") ||
		!strings.Contains(archive.NameTemplate, ".Os") ||
		!strings.Contains(archive.NameTemplate, ".Arch") {
		return false
	}
	for _, override := range archive.FormatOverrides {
		if override.Goos == "windows" && contains(override.Formats, "zip") {
			return true
		}
	}
	return false
}

// PluginArtifactSupportsInstallation verifies the focused plugin build and
// archive shape used by the existing plugin installer contract.
func PluginArtifactSupportsInstallation(config Config, binaryName, assetPrefix string) bool {
	build, buildOK := BuildByID(config.Builds, binaryName)
	archive, archiveOK := ArchiveByID(config.Archives, assetPrefix)
	return buildOK && archiveOK && build.Binary == binaryName &&
		contains(archive.IDs, build.ID) &&
		contains(archive.Formats, "tar.gz") &&
		strings.Contains(archive.NameTemplate, assetPrefix+"_") &&
		strings.Contains(archive.NameTemplate, ".Os") && strings.Contains(archive.NameTemplate, ".Arch")
}

// PublicationAssets derives the exact platform archive and checksum names used
// by the existing Release Plugin remote verification contract.
func PublicationAssets(config Config, prefix, version string, plugin bool) []string {
	formats := PlatformFormats{Darwin: "tar.gz", Linux: "tar.gz", Windows: "tar.gz"}
	archive, present := ArchiveByID(config.Archives, prefix)
	if !present {
		return nil
	}
	for _, override := range archive.FormatOverrides {
		if len(override.Formats) == 0 {
			continue
		}
		switch override.Goos {
		case "darwin":
			formats.Darwin = override.Formats[0]
		case "linux":
			formats.Linux = override.Formats[0]
		case "windows":
			formats.Windows = override.Formats[0]
		}
	}
	nameVersion := ""
	if plugin {
		nameVersion = version
	}
	assets := PlatformArchiveAssets(prefix, nameVersion, formats)
	if config.Checksum != nil {
		assets = append(assets, prefix+"_"+version+"_checksums.txt")
	}
	sort.Strings(assets)
	return assets
}

// PlatformArchiveAssets derives deterministic archive names for the supported
// installer platforms and architectures.
func PlatformArchiveAssets(prefix, version string, formats PlatformFormats) []string {
	platforms := []struct {
		name   string
		format string
	}{
		{name: "Darwin", format: formats.Darwin},
		{name: "Linux", format: formats.Linux},
		{name: "Windows", format: formats.Windows},
	}
	assets := make([]string, 0, len(platforms)*3)
	for _, platform := range platforms {
		format := platform.format
		if format == "" {
			continue
		}
		for _, architecture := range []string{"arm64", "i386", "x86_64"} {
			// Current Go toolchains and GoReleaser exclude darwin/386 from the
			// configured build matrix, so it must never become a required asset.
			if platform.name == "Darwin" && architecture == "i386" {
				continue
			}
			parts := []string{prefix}
			if version != "" {
				parts = append(parts, version)
			}
			parts = append(parts, platform.name, architecture)
			assets = append(assets, strings.Join(parts, "_")+"."+format)
		}
	}
	sort.Strings(assets)
	return assets
}
