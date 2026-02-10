package update

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/git"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/version"
	"golang.org/x/mod/semver"
)

// CoreOptions holds options for core update
type CoreOptions struct {
	Force  bool
	DryRun bool
}

// Core handles updating the neko-cli core tool
func Core(opts CoreOptions) error {
	log.Print(log.Init, "Checking for neko-cli updates...")

	// Check if running a development build
	if isDevelopmentBuild(version.Version) {
		log.Print(log.Exec, fmt.Sprintf("You are running a development build (%s)", version.Version))
		log.Print(log.Exec, "Development builds cannot be updated automatically. Please build from source or download a release.")
		return nil
	}

	// Get repository info for neko-cli
	repoInfo := &github.RepoInfo{
		Owner: "nekoman-hq",
		Repo:  "neko-cli",
	}

	// Get latest release
	release, err := github.LatestRelease(repoInfo)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if release == nil {
		return fmt.Errorf("no releases found for repository %s/%s - check if you internet connection is working and if the repository has releases",
			repoInfo.Owner, repoInfo.Repo)
	}

	// Compare versions
	currentVersion := normalizeVersion(version.Version)
	latestVersion := normalizeVersion(release.TagName)

	// Check if update is needed
	needsUpdate := semver.Compare(currentVersion, latestVersion) < 0

	if !needsUpdate && !opts.Force {
		log.Print(log.Exec, fmt.Sprintf("You are already running the latest version (%s)", version.Version))
		return nil
	}

	if needsUpdate {
		log.Print(log.Exec, fmt.Sprintf("Current version: %s → Latest version: %s",
			version.Version, strings.TrimPrefix(latestVersion, "v")))
	} else {
		log.Print(log.Exec, fmt.Sprintf("Forcing update to %s (same version)",
			strings.TrimPrefix(latestVersion, "v")))
	}

	// Dry run mode
	if opts.DryRun {
		log.Print(log.Exec, "Update check complete (dry-run mode, not installing)")
		return nil
	}

	// Download and install the update
	if err := downloadAndInstallCore(release); err != nil {
		return fmt.Errorf("failed to install update: %w", err)
	}

	log.Print(log.Exec, fmt.Sprintf("Successfully updated to version %s",
		strings.TrimPrefix(latestVersion, "v")))

	return nil
}

// isDevelopmentBuild checks if the version string indicates a development build
func isDevelopmentBuild(v string) bool {
	// Check for common development version patterns:
	// - Contains "dirty" (uncommitted changes)
	// - Contains git hash (e.g., "v2.1.9-1-gb615e10")
	// - Contains "dev" or "devel"
	return strings.Contains(v, "dirty") ||
		strings.Contains(v, "-g") || // git hash indicator
		strings.Contains(v, "dev") ||
		strings.Contains(v, "devel")
}

// normalizeVersion ensures version has 'v' prefix for semver comparison
func normalizeVersion(v string) string {
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// downloadAndInstallCore downloads and installs a new version of neko-cli
func downloadAndInstallCore(release *github.Release) error {
	downloadURL, err := getDownloadURL(release)
	if err != nil {
		return err
	}

	log.Print(log.Exec, fmt.Sprintf("Downloading from: %s", downloadURL))

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download release: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Print(log.Exec, fmt.Sprintf("Warning: failed to close response body: %v", err))
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download release: HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "neko-update-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			log.Print(log.Exec, fmt.Sprintf("Warning: failed to remove temp file %s: %v", name, err))
		}
	}(tmpName)

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		err := tmpFile.Close()
		if err != nil {
			return err
		}
		return fmt.Errorf("failed to download file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := extractAndInstall(tmpName); err != nil {
		return fmt.Errorf("failed to extract and install: %w", err)
	}

	return nil
}

// extractAndInstall extracts the downloaded archive and installs the binary
func extractAndInstall(archivePath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			log.Print(log.Exec, fmt.Sprintf("Warning: failed to close archive file: %v", err))
		}
	}(file)

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func(gzr *gzip.Reader) {
		err = gzr.Close()
		if err != nil {
			log.Print(log.Exec, fmt.Sprintf("Warning: failed to close gzip reader: %v", err))
		}
	}(gzr)

	tr := tar.NewReader(gzr)

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	log.Print(log.Exec, fmt.Sprintf("Installing to: %s", currentExe))

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		baseName := filepath.Base(header.Name)
		if baseName == "neko" || baseName == "neko.exe" || baseName == "neko-cli" || baseName == "neko-cli.exe" {
			return installBinary(tr, currentExe)
		}
	}

	return fmt.Errorf("neko binary not found in archive")
}

// installBinary installs the binary with backup and rollback support
func installBinary(reader io.Reader, targetPath string) error {
	backupPath := targetPath + ".backup"

	if err := copyFile(targetPath, backupPath); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return fmt.Errorf(
				"permission denied while updating %s.\n\nTry running:\n\nsudo neko update --force",
				targetPath,
			)
		}
		return fmt.Errorf("failed to create backup: %w", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			log.Print(log.Exec, fmt.Sprintf("Warning: failed to remove backup file %s: %v", name, err))
		}
	}(backupPath)

	tmpBinary, err := os.CreateTemp(filepath.Dir(targetPath), "neko-new-*")
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return fmt.Errorf(
				"permission denied while writing to %s.\n\nTry running:\n\nsudo neko update --force",
				filepath.Dir(targetPath),
			)
		}
		return fmt.Errorf("failed to create temp binary: %w", err)
	}

	tmpPath := tmpBinary.Name()
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			log.Print(log.Exec, fmt.Sprintf("Warning: failed to remove temp binary %s: %v", name, err))
		}
	}(tmpPath)

	if _, err := io.Copy(tmpBinary, reader); err != nil {
		err := tmpBinary.Close()
		if err != nil {
			return err
		}
		return fmt.Errorf("failed to extract binary: %w", err)
	}

	if err := tmpBinary.Close(); err != nil {
		return fmt.Errorf("failed to close temp binary: %w", err)
	}

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return fmt.Errorf(
				"permission denied while replacing %s.\n\nTry running:\n\nsudo neko update --force",
				targetPath,
			)
		}

		if restoreErr := copyFile(backupPath, targetPath); restoreErr != nil {
			return fmt.Errorf("failed to replace binary: %w (rollback also failed: %v)", err, restoreErr)
		}
		return fmt.Errorf("failed to replace binary (rolled back): %w", err)
	}

	return nil
}

// getDownloadURL finds the appropriate download URL for the current platform
func getDownloadURL(release *github.Release) (string, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	archName := mapArchName(arch)
	osNameCap := strings.ToUpper(osName[:1]) + osName[1:]
	assetPattern := fmt.Sprintf("neko-cli_%s_%s", osNameCap, archName)

	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, assetPattern) && strings.HasSuffix(asset.Name, ".tar.gz") {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("no compatible release found for %s/%s (expected pattern: %s*.tar.gz)",
		osName, arch, assetPattern)
}

// mapArchName maps Go architecture names to goreleaser naming conventions
func mapArchName(arch string) string {
	switch arch {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	default:
		return arch
	}
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func(sourceFile *os.File) {
		err = sourceFile.Close()
		if err != nil {
			log.Print(log.Exec, fmt.Sprintf("Warning: failed to close source file: %v", err))
		}
	}(sourceFile)

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func(destFile *os.File) {
		err := destFile.Close()
		if err != nil {
			log.Print(log.Exec, fmt.Sprintf("Warning: failed to close destination file: %v", err))
		}
	}(destFile)

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	if err := destFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	return nil
}
