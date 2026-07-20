package goreleaser

import (
	"reflect"
	"testing"
)

func TestVerifyArtifactContractReturnsNeutralOrderedFindings(t *testing.T) {
	expectation := ArtifactExpectation{UnitID: "plugin-release", BuildID: "plugin-release", Binary: "plugin-release", ChecksumPrefix: "plugin-release", Plugin: true}
	config := Config{
		Version:     2,
		ProjectName: "neko",
		Builds: []Build{{
			ID: "plugin-release", Binary: "wrong", Goos: []string{"linux"},
		}},
		Archives: []Archive{{
			ID: "plugin-release", IDs: []string{"plugin-release"}, Formats: []string{"zip"}, NameTemplate: "wrong",
		}},
	}
	want := []Finding{
		{Kind: FindingBinaryMismatch, UnitID: "plugin-release", ExpectedID: "plugin-release", ExpectedBinary: "plugin-release"},
		{Kind: FindingPlatformMismatch, UnitID: "plugin-release", ExpectedID: "plugin-release"},
		{Kind: FindingArchiveMismatch, UnitID: "plugin-release", ExpectedID: "plugin-release"},
		{Kind: FindingChecksumMissing, UnitID: "plugin-release", ExpectedID: "plugin-release"},
		{Kind: FindingReleaseIdentityMismatch, UnitID: "plugin-release", ExpectedID: "plugin-release"},
	}
	if got := VerifyArtifactContract(config, []ArtifactExpectation{expectation}); !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
}

func TestPublicationAssetsUseArchiveFormatOverrides(t *testing.T) {
	config := Config{
		Archives: []Archive{{
			ID:              "plugin-release",
			FormatOverrides: []FormatOverride{{Goos: "windows", Formats: []string{"zip"}}},
		}},
		Checksum: &Checksum{},
	}
	assets := PublicationAssets(config, "plugin-release", "1.2.3", true)
	for _, want := range []string{
		"plugin-release_1.2.3_Darwin_arm64.tar.gz",
		"plugin-release_1.2.3_Linux_x86_64.tar.gz",
		"plugin-release_1.2.3_Windows_i386.zip",
		"plugin-release_1.2.3_checksums.txt",
	} {
		if !Contains(assets, want) {
			t.Errorf("assets %v omit %q", assets, want)
		}
	}
}
