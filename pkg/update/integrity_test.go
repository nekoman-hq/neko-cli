package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	github "github.com/nekoman-hq/neko-cli/pkg/git"
)

func TestSelectReleaseAssetsRequiresOneArchiveAndChecksum(t *testing.T) {
	release := &github.Release{
		TagName: "v1.2.3",
		Assets: []github.Asset{
			{Name: "neko-cli_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.invalid/archive"},
			{Name: "neko-cli_1.2.3_checksums.txt", BrowserDownloadURL: "https://example.invalid/checksums"},
		},
	}
	assets, err := selectReleaseAssets(release, platform{OS: "darwin", Arch: "arm64"})
	if err != nil {
		t.Fatalf("selectReleaseAssets: %v", err)
	}
	if assets.archive.Name != "neko-cli_Darwin_arm64.tar.gz" || assets.checksum.Name != "neko-cli_1.2.3_checksums.txt" {
		t.Fatalf("assets = %#v", assets)
	}

	for _, test := range []struct {
		name      string
		mutate    func(*github.Release)
		wantError string
	}{
		{name: "missing archive", mutate: func(release *github.Release) { release.Assets = release.Assets[1:] }, wantError: "no compatible archive"},
		{name: "missing checksum", mutate: func(release *github.Release) { release.Assets = release.Assets[:1] }, wantError: "no authoritative checksum"},
		{name: "ambiguous archive", mutate: func(release *github.Release) {
			release.Assets = append(release.Assets, github.Asset{Name: "neko-cli_1.2.3_Darwin_arm64.tar.gz"})
		}, wantError: "multiple compatible archives"},
	} {
		t.Run(test.name, func(t *testing.T) {
			copyRelease := *release
			copyRelease.Assets = append([]github.Asset(nil), release.Assets...)
			test.mutate(&copyRelease)
			_, err := selectReleaseAssets(&copyRelease, platform{OS: "darwin", Arch: "arm64"})
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want %q", err, test.wantError)
			}
		})
	}
}

func TestSelectReleaseAssetsRejectsUnsupportedPlatforms(t *testing.T) {
	for _, target := range []platform{{OS: "windows", Arch: "amd64"}, {OS: "darwin", Arch: "386"}, {OS: "plan9", Arch: "arm64"}} {
		_, err := selectReleaseAssets(&github.Release{TagName: "v1.2.3"}, target)
		if err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("platform %#v error = %v", target, err)
		}
	}
}

func TestVerifyArchiveChecksum(t *testing.T) {
	archive := []byte("frozen archive")
	digest := sha256.Sum256(archive)
	valid := fmt.Sprintf("%x  other.tar.gz\n%x  neko-cli_Darwin_arm64.tar.gz\n", sha256.Sum256([]byte("other")), digest)
	if err := verifyArchiveChecksum("neko-cli_Darwin_arm64.tar.gz", archive, []byte(valid)); err != nil {
		t.Fatalf("valid checksum: %v", err)
	}

	tests := []struct { //nolint:govet // Field order keeps archive cases readable.
		name      string
		manifest  string
		wantError string
	}{
		{name: "missing", manifest: fmt.Sprintf("%x  other.tar.gz\n", digest), wantError: "no entry"},
		{name: "duplicate", manifest: fmt.Sprintf("%x  neko-cli_Darwin_arm64.tar.gz\n%x  neko-cli_Darwin_arm64.tar.gz\n", digest, digest), wantError: "duplicate"},
		{name: "malformed", manifest: "not-a-checksum", wantError: "must contain"},
		{name: "invalid digest", manifest: "xyz  neko-cli_Darwin_arm64.tar.gz", wantError: "invalid SHA-256"},
		{name: "mismatch", manifest: fmt.Sprintf("%x  neko-cli_Darwin_arm64.tar.gz\n", sha256.Sum256([]byte("different"))), wantError: "mismatch"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := verifyArchiveChecksum("neko-cli_Darwin_arm64.tar.gz", archive, []byte(test.manifest))
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want %q", err, test.wantError)
			}
		})
	}
}

func TestExtractCoreBinaryValidatesEveryEntry(t *testing.T) {
	valid := archiveFixture(t, tarFixture{name: "dist/neko-cli", body: []byte("binary"), typeFlag: tar.TypeReg})
	binary, err := extractCoreBinary(valid)
	if err != nil || string(binary) != "binary" {
		t.Fatalf("valid archive: binary=%q err=%v", binary, err)
	}

	tests := []struct { //nolint:govet // Field order keeps archive cases readable.
		name      string
		archive   []byte
		wantError string
	}{
		{name: "malformed gzip", archive: []byte("invalid"), wantError: "not valid gzip"},
		{name: "path traversal", archive: archiveFixture(t, tarFixture{name: "../neko", body: []byte("binary"), typeFlag: tar.TypeReg}), wantError: "unsafe path"},
		{name: "symlink", archive: archiveFixture(t, tarFixture{name: "neko", typeFlag: tar.TypeSymlink}), wantError: "not a regular file"},
		{name: "multiple", archive: archiveFixture(t,
			tarFixture{name: "neko", body: []byte("one"), typeFlag: tar.TypeReg},
			tarFixture{name: "nested/neko-cli", body: []byte("two"), typeFlag: tar.TypeReg},
		), wantError: "multiple"},
		{name: "missing", archive: archiveFixture(t, tarFixture{name: "README", body: []byte("text"), typeFlag: tar.TypeReg}), wantError: "no supported"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := extractCoreBinary(test.archive)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want %q", err, test.wantError)
			}
		})
	}
}

type tarFixture struct {
	name     string
	body     []byte
	typeFlag byte
}

func archiveFixture(t *testing.T, entries ...tarFixture) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o755, Size: int64(len(entry.body)), Typeflag: entry.typeFlag}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if len(entry.body) > 0 {
			if _, err := tarWriter.Write(entry.body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
