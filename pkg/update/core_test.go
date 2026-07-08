package update

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/git"
)

func TestGetDownloadURLMatchesVersionedGoReleaserAsset(t *testing.T) {
	osName := releaseOSName(runtime.GOOS)
	archName := mapArchName(runtime.GOARCH)
	assetName := fmt.Sprintf("neko-cli_3.0.1_%s_%s.tar.gz", osName, archName)
	release := &github.Release{
		Assets: []github.Asset{
			{Name: "neko-cli_3.0.1_checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
			{Name: assetName, BrowserDownloadURL: "https://example.com/neko-cli.tar.gz"},
		},
	}

	url, err := getDownloadURL(release)
	if err != nil {
		t.Fatalf("getDownloadURL returned error: %v", err)
	}
	if url != "https://example.com/neko-cli.tar.gz" {
		t.Fatalf("getDownloadURL = %q, want versioned asset URL", url)
	}
}

func TestIsCompatibleCoreReleaseAssetSupportsLegacyAndVersionedNames(t *testing.T) {
	tests := []struct {
		name  string
		match bool
	}{
		{name: "neko-cli_Darwin_arm64.tar.gz", match: true},
		{name: "neko-cli_3.0.1_Darwin_arm64.tar.gz", match: true},
		{name: "neko-cli_3.0.1_Darwin_x86_64.tar.gz", match: false},
		{name: "neko-cli_3.0.1_checksums.txt", match: false},
	}

	for _, tt := range tests {
		got := isCompatibleCoreReleaseAsset(tt.name, "Darwin", "arm64")
		if got != tt.match {
			t.Fatalf("isCompatibleCoreReleaseAsset(%q) = %v, want %v", tt.name, got, tt.match)
		}
	}
}
