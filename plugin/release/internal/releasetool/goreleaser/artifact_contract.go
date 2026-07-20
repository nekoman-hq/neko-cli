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
		if !ContainsAll(build.Goos, "darwin", "linux", "windows") {
			findings = append(findings, Finding{Kind: FindingPlatformMismatch, UnitID: expectation.UnitID, ExpectedID: expectation.BuildID})
		}
		archive, archiveOK := ArchiveByID(config.Archives, expectation.BuildID)
		if !archiveOK || !Contains(archive.IDs, expectation.BuildID) {
			findings = append(findings, Finding{Kind: FindingArchiveIdentityMismatch, UnitID: expectation.UnitID, ExpectedID: expectation.BuildID})
			continue
		}
		if !Contains(archive.Formats, "tar.gz") ||
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
		if !Contains(config.Release.IDs, expectation.BuildID) {
			findings = append(findings, Finding{Kind: FindingReleaseIdentityMismatch, UnitID: expectation.UnitID, ExpectedID: expectation.BuildID})
		}
	}
	return findings
}

// PublicationAssets derives the exact platform archive and checksum names used
// by the existing Release Plugin remote verification contract.
func PublicationAssets(config Config, prefix, version string, plugin bool) []string {
	formats := map[string]string{"Darwin": "tar.gz", "Linux": "tar.gz", "Windows": "tar.gz"}
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
			formats["Darwin"] = override.Formats[0]
		case "linux":
			formats["Linux"] = override.Formats[0]
		case "windows":
			formats["Windows"] = override.Formats[0]
		}
	}
	nameVersion := ""
	if plugin {
		nameVersion = version
	}
	assets := platformArchiveAssets(prefix, nameVersion, formats)
	if config.Checksum != nil {
		assets = append(assets, prefix+"_"+version+"_checksums.txt")
	}
	sort.Strings(assets)
	return assets
}

func platformArchiveAssets(prefix, version string, formats map[string]string) []string {
	assets := make([]string, 0, len(formats)*3)
	for _, operatingSystem := range []string{"Darwin", "Linux", "Windows"} {
		format := formats[operatingSystem]
		if format == "" {
			continue
		}
		for _, architecture := range []string{"arm64", "i386", "x86_64"} {
			parts := []string{prefix}
			if version != "" {
				parts = append(parts, version)
			}
			parts = append(parts, operatingSystem, architecture)
			assets = append(assets, strings.Join(parts, "_")+"."+format)
		}
	}
	sort.Strings(assets)
	return assets
}
