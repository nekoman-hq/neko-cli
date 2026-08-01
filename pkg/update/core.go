package update

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	github "github.com/nekoman-hq/neko-cli/pkg/git"
	"github.com/nekoman-hq/neko-cli/pkg/version"
)

// CoreOptions holds user intent for the Core self-update operation.
type CoreOptions struct {
	Force  bool
	DryRun bool
}

// CoreResult reports the completed action without owning command rendering.
type CoreResult struct {
	Action             Action
	InstalledVersion   string
	SelectedVersion    string
	RunningExecutable  string
	CanonicalTarget    string
	Installation       string
	Manager            string
	DryRun             bool
	DevelopmentBuild   bool
	ArchiveRequested   bool
	DestinationChanged bool
	CapabilityWarning  string
}

type releaseClient interface {
	LatestRelease(context.Context, *github.RepoInfo) (*github.Release, error)
	Download(context.Context, string, int64) ([]byte, error)
}

type coreDependencies struct {
	installedVersion string
	releases         releaseClient
	inspector        installationInspector
	replacement      replacementCapability
	platform         platform
}

// ExecuteCore plans and executes a self-update using context-bound I/O.
func ExecuteCore(ctx context.Context, opts CoreOptions) (CoreResult, error) {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return executeCore(ctx, opts, coreDependencies{
		installedVersion: version.Version,
		releases:         github.NewClient(httpClient),
		inspector:        newOSInstallationInspector(),
		replacement:      newOSReplacementCapability(),
		platform:         platform{OS: runtime.GOOS, Arch: runtime.GOARCH},
	})
}

// Core retains the established package entry point for callers that do not
// need the structured result. Command code should use ExecuteCore.
func Core(opts CoreOptions) error {
	_, err := ExecuteCore(context.Background(), opts)
	return err
}

func executeCore(ctx context.Context, opts CoreOptions, deps coreDependencies) (result CoreResult, returnErr error) {
	result.InstalledVersion = deps.installedVersion
	result.DryRun = opts.DryRun
	if isDevelopmentBuild(deps.installedVersion) {
		result.Action = ActionUnsupported
		result.DevelopmentBuild = true
		return result, nil
	}

	release, err := deps.releases.LatestRelease(ctx, &github.RepoInfo{Owner: "nekoman-hq", Repo: "neko-cli"})
	if err != nil {
		return result, newUpdateError(errorReleaseLookup, "failed to check for neko-cli updates; no archive was downloaded and the installed executable is unchanged", err)
	}
	if release == nil {
		return result, newUpdateError(errorReleaseLookup, "no stable neko-cli release was selected; no archive was downloaded and the installed executable is unchanged", nil)
	}
	result.SelectedVersion = strings.TrimPrefix(release.TagName, "v")
	action, actionErr := classifyUpdateAction(deps.installedVersion, release.TagName, opts.Force)
	result.Action = action
	if actionErr != nil {
		return result, newUpdateError(
			errorDowngrade,
			fmt.Sprintf("installed neko-cli %s is newer than selected latest %s; force is not a downgrade flag; no archive was downloaded and the installed executable is unchanged", deps.installedVersion, result.SelectedVersion),
			actionErr,
		)
	}

	currentInstallation, err := deps.inspector.Inspect()
	if err != nil {
		return result, err
	}
	result.RunningExecutable = currentInstallation.runningExecutable
	result.CanonicalTarget = currentInstallation.canonicalTarget
	result.Installation = string(currentInstallation.classification)
	result.Manager = currentInstallation.manager

	downloadRequired := action == ActionUpgrade || action == ActionForcedReinstall
	plan := updatePlan{
		installedVersion:     deps.installedVersion,
		selectedVersion:      result.SelectedVersion,
		action:               action,
		runningExecutable:    currentInstallation.runningExecutable,
		symlinkPath:          currentInstallation.symlinkPath,
		canonicalTarget:      currentInstallation.canonicalTarget,
		targetParent:         currentInstallation.targetParent,
		classification:       currentInstallation.classification,
		manager:              currentInstallation.manager,
		managerGuidance:      currentInstallation.managerGuidance,
		targetMode:           currentInstallation.targetMode,
		downloadRequired:     downloadRequired,
		replacementPermitted: currentInstallation.targetReadable && currentInstallation.parentCreateAllowed && currentInstallation.parentReplaceAllowed && currentInstallation.classification != installationManagerOwned,
	}

	if !downloadRequired {
		return result, nil
	}

	if currentInstallation.classification == installationManagerOwned {
		plan = plan.withRefusal(
			fmt.Sprintf("%s owns %s", currentInstallation.manager, currentInstallation.canonicalTarget),
			currentInstallation.managerGuidance,
		)
		result.CapabilityWarning = managerOwnedMessage(plan)
		if opts.DryRun {
			return result, nil
		}
		return result, newUpdateError(errorManagerOwned, result.CapabilityWarning, nil)
	}

	if !currentInstallation.parentCreateAllowed || !currentInstallation.parentReplaceAllowed {
		plan = plan.withRefusal(
			fmt.Sprintf("parent directory %s is not writable by the current user", currentInstallation.targetParent),
			permissionGuidance(plan, opts.Force),
		)
		result.CapabilityWarning = permissionMessage(plan, opts.Force)
		if opts.DryRun {
			return result, nil
		}
		return result, newUpdateError(errorParentNotWritable, result.CapabilityWarning, nil)
	}

	assets, err := selectReleaseAssets(release, deps.platform)
	if err != nil {
		return result, err
	}
	plan = plan.withAssets(assets)
	if opts.DryRun {
		return result, nil
	}

	reservation, err := deps.replacement.Reserve(currentInstallation)
	if err != nil {
		return result, err
	}
	defer func() {
		cleanupErr := reservation.Cleanup()
		if cleanupErr == nil {
			return
		}
		if returnErr == nil {
			cleanupFailure := newUpdateError(errorCleanup, fmt.Sprintf("update cleanup failed for %s", currentInstallation.canonicalTarget), cleanupErr)
			cleanupFailure.changed = result.DestinationChanged
			returnErr = cleanupFailure
			return
		}
		var updateErr *updateError
		if errors.As(returnErr, &updateErr) {
			updateErr.cleanup = cleanupErr.Error()
			return
		}
		returnErr = fmt.Errorf("%w; cleanup: %v", returnErr, cleanupErr)
	}()

	checksumManifest, err := deps.releases.Download(ctx, assets.checksum.BrowserDownloadURL, maxChecksumBytes)
	if err != nil {
		return result, newUpdateError(errorDownload, fmt.Sprintf("failed to download checksum asset %s; no archive was downloaded and the installed executable is unchanged", assets.checksum.Name), err)
	}
	result.ArchiveRequested = true
	archive, err := deps.releases.Download(ctx, assets.archive.BrowserDownloadURL, maxArchiveBytes)
	if err != nil {
		downloadError := newUpdateError(errorDownload, fmt.Sprintf("failed to download release archive %s; the installed executable is unchanged", assets.archive.Name), err)
		downloadError.downloaded = true
		return result, downloadError
	}
	if err := verifyArchiveChecksum(assets.archive.Name, archive, checksumManifest); err != nil {
		if updateErr, ok := err.(*updateError); ok {
			updateErr.downloaded = true
		}
		return result, err
	}
	binary, err := extractCoreBinary(archive)
	if err != nil {
		if updateErr, ok := err.(*updateError); ok {
			updateErr.downloaded = true
		}
		return result, err
	}
	if err := reservation.Commit(binary, currentInstallation.targetMode); err != nil {
		var updateErr *updateError
		if errors.As(err, &updateErr) {
			updateErr.downloaded = true
			result.DestinationChanged = updateErr.changed
		}
		return result, err
	}
	result.DestinationChanged = true
	return result, nil
}

func managerOwnedMessage(plan updatePlan) string {
	return fmt.Sprintf(
		"refusing to self-update %s because it is managed by %s; the installed executable is unchanged and no archive was downloaded; use %s",
		plan.canonicalTarget,
		plan.manager,
		plan.managerGuidance,
	)
}

func permissionMessage(plan updatePlan, force bool) string {
	return fmt.Sprintf(
		"cannot update %s: parent directory %s is not writable by the current user; atomic replacement requires create, rename, and remove capability in that directory; no archive was downloaded and the installed executable is unchanged; %s",
		plan.canonicalTarget,
		plan.targetParent,
		permissionGuidance(plan, force),
	)
}

func permissionGuidance(plan updatePlan, force bool) string {
	primary := "reinstall neko-cli into a user-owned directory such as $HOME/.local/bin"
	command := fmt.Sprintf("sudo -- %s update", shellQuote(plan.runningExecutable))
	if force && plan.action == ActionForcedReinstall {
		command += " --force"
	}
	return fmt.Sprintf("%s; as a deliberate secondary choice for this unmanaged system installation, rerun %s", primary, command)
}

func shellQuote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\n'\"\\$`!&|;<>()[]{}*?") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

// isDevelopmentBuild preserves the established development-build policy.
func isDevelopmentBuild(value string) bool {
	return strings.Contains(value, "dirty") ||
		strings.Contains(value, "-g") ||
		strings.Contains(value, "dev") ||
		strings.Contains(value, "devel")
}

func normalizeVersion(value string) string {
	if !strings.HasPrefix(value, "v") {
		return "v" + value
	}
	return value
}
