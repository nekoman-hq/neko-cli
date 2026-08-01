package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"

	github "github.com/nekoman-hq/neko-cli/pkg/git"
)

const (
	maxArchiveBytes    int64 = 256 << 20
	maxChecksumBytes   int64 = 2 << 20
	maxExecutableBytes int64 = 256 << 20
)

type releaseAssets struct {
	archive  github.Asset
	checksum github.Asset
}

func selectReleaseAssets(release *github.Release, target platform) (releaseAssets, error) {
	osName, archName, err := supportedReleasePlatform(target)
	if err != nil {
		return releaseAssets{}, err
	}
	version := strings.TrimPrefix(release.TagName, "v")
	archiveNames := map[string]bool{
		fmt.Sprintf("neko-cli_%s_%s.tar.gz", osName, archName):             true,
		fmt.Sprintf("neko-cli_%s_%s_%s.tar.gz", version, osName, archName): true,
	}
	checksumNames := map[string]bool{
		fmt.Sprintf("neko-cli_%s_checksums.txt", version): true,
		"checksums.txt": true,
	}

	var archives []github.Asset
	var checksums []github.Asset
	for _, asset := range release.Assets {
		if archiveNames[asset.Name] {
			archives = append(archives, asset)
		}
		if checksumNames[asset.Name] {
			checksums = append(checksums, asset)
		}
	}
	if len(archives) == 0 {
		return releaseAssets{}, newUpdateError(
			errorAssetMissing,
			fmt.Sprintf("release %s has no compatible archive for %s/%s", release.TagName, target.OS, target.Arch),
			nil,
		)
	}
	if len(archives) != 1 {
		return releaseAssets{}, newUpdateError(errorAssetMissing, fmt.Sprintf("release %s has multiple compatible archives for %s/%s", release.TagName, target.OS, target.Arch), nil)
	}
	if len(checksums) == 0 {
		return releaseAssets{}, newUpdateError(errorChecksumMissing, fmt.Sprintf("release %s has no authoritative checksum asset", release.TagName), nil)
	}
	if len(checksums) != 1 {
		return releaseAssets{}, newUpdateError(errorChecksumMissing, fmt.Sprintf("release %s has multiple authoritative checksum assets", release.TagName), nil)
	}
	return releaseAssets{archive: archives[0], checksum: checksums[0]}, nil
}

func supportedReleasePlatform(target platform) (string, string, error) {
	var osName string
	switch target.OS {
	case "darwin":
		osName = "Darwin"
	case "linux":
		osName = "Linux"
	default:
		return "", "", newUpdateError(errorUnsupportedPlatform, fmt.Sprintf("self-update is unsupported on %s/%s", target.OS, target.Arch), nil)
	}

	var archName string
	switch target.Arch {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "arm64"
	default:
		return "", "", newUpdateError(errorUnsupportedPlatform, fmt.Sprintf("self-update is unsupported on %s/%s", target.OS, target.Arch), nil)
	}
	return osName, archName, nil
}

func verifyArchiveChecksum(archiveName string, archive, manifest []byte) error {
	var expected []byte
	matches := 0
	for lineNumber, line := range strings.Split(string(manifest), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return newUpdateError(errorChecksumInvalid, fmt.Sprintf("checksum manifest line %d must contain a SHA-256 digest and filename", lineNumber+1), nil)
		}
		digest, err := hex.DecodeString(fields[0])
		if err != nil || len(digest) != sha256.Size {
			return newUpdateError(errorChecksumInvalid, fmt.Sprintf("checksum manifest line %d has an invalid SHA-256 digest", lineNumber+1), err)
		}
		filename := strings.TrimPrefix(fields[1], "*")
		if filename != archiveName {
			continue
		}
		matches++
		expected = digest
	}
	if matches == 0 {
		return newUpdateError(errorChecksumInvalid, fmt.Sprintf("checksum manifest has no entry for %s", archiveName), nil)
	}
	if matches != 1 {
		return newUpdateError(errorChecksumInvalid, fmt.Sprintf("checksum manifest has duplicate entries for %s", archiveName), nil)
	}
	actual := sha256.Sum256(archive)
	if subtle.ConstantTimeCompare(expected, actual[:]) != 1 {
		return newUpdateError(errorChecksumMismatch, fmt.Sprintf("checksum mismatch for %s", archiveName), nil)
	}
	return nil
}

func extractCoreBinary(archive []byte) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, newUpdateError(errorArchiveInvalid, "release archive is not valid gzip", err)
	}
	defer func() { _ = gzipReader.Close() }()

	tarReader := tar.NewReader(gzipReader)
	var binary []byte
	matches := 0
	for {
		header, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, newUpdateError(errorArchiveInvalid, "release archive contains invalid tar data", nextErr)
		}
		if unsafeArchivePath(header.Name) {
			return nil, newUpdateError(errorArchiveInvalid, fmt.Sprintf("release archive contains unsafe path %q", header.Name), nil)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return nil, newUpdateError(errorArchiveInvalid, fmt.Sprintf("release archive entry %q is not a regular file", header.Name), nil)
		}
		if !supportedBinaryName(path.Base(header.Name)) {
			continue
		}
		matches++
		if matches > 1 {
			return nil, newUpdateError(errorArchiveInvalid, "release archive contains multiple neko binaries", nil)
		}
		if header.Size <= 0 || header.Size > maxExecutableBytes {
			return nil, newUpdateError(errorArchiveInvalid, fmt.Sprintf("release archive binary size %d is invalid", header.Size), nil)
		}
		content, readErr := io.ReadAll(io.LimitReader(tarReader, maxExecutableBytes+1))
		if readErr != nil {
			return nil, newUpdateError(errorArchiveInvalid, "cannot read release archive binary", readErr)
		}
		if int64(len(content)) != header.Size || int64(len(content)) > maxExecutableBytes {
			return nil, newUpdateError(errorArchiveInvalid, "release archive binary is truncated or exceeds the size limit", nil)
		}
		binary = content
	}
	if matches == 0 {
		return nil, newUpdateError(errorArchiveInvalid, "release archive contains no supported neko binary", nil)
	}
	return binary, nil
}

func unsafeArchivePath(name string) bool {
	if name == "" || strings.Contains(name, "\\") || path.IsAbs(name) {
		return true
	}
	clean := path.Clean(name)
	return clean == "." || clean == ".." || strings.HasPrefix(clean, "../")
}

func supportedBinaryName(name string) bool {
	return name == "neko" || name == "neko-cli"
}
