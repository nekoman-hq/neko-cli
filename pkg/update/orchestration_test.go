package update

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	github "github.com/nekoman-hq/neko-cli/pkg/git"
)

func TestExecuteCoreForceAndVersionMatrix(t *testing.T) {
	tests := []struct { //nolint:govet // Field order mirrors the version-policy matrix.
		name          string
		installed     string
		available     string
		force         bool
		wantAction    Action
		wantError     bool
		wantDownloads int
		wantChanged   bool
	}{
		{name: "upgrade", installed: "1.0.0", available: "1.1.0", wantAction: ActionUpgrade, wantDownloads: 2, wantChanged: true},
		{name: "forced upgrade", installed: "1.0.0", available: "1.1.0", force: true, wantAction: ActionUpgrade, wantDownloads: 2, wantChanged: true},
		{name: "already current", installed: "1.1.0", available: "1.1.0", wantAction: ActionAlreadyCurrent},
		{name: "forced reinstall", installed: "1.1.0", available: "1.1.0", force: true, wantAction: ActionForcedReinstall, wantDownloads: 2, wantChanged: true},
		{name: "installed newer", installed: "1.2.0", available: "1.1.0", wantAction: ActionInstalledNewer},
		{name: "forced downgrade refused", installed: "1.2.0", available: "1.1.0", force: true, wantAction: ActionDowngradeRefused, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deps, client, target, oldContent := coreFixture(t, test.installed, test.available)
			result, err := executeCore(context.Background(), CoreOptions{Force: test.force}, deps)
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, wantError=%t", err, test.wantError)
			}
			if result.Action != test.wantAction {
				t.Fatalf("action = %q, want %q", result.Action, test.wantAction)
			}
			if len(client.downloads) != test.wantDownloads {
				t.Fatalf("asset downloads = %d, want %d (%v)", len(client.downloads), test.wantDownloads, client.downloads)
			}
			content, readErr := os.ReadFile(target)
			if readErr != nil {
				t.Fatal(readErr)
			}
			changed := string(content) != string(oldContent)
			if changed != test.wantChanged || result.DestinationChanged != test.wantChanged {
				t.Fatalf("changed=%t result.changed=%t want=%t", changed, result.DestinationChanged, test.wantChanged)
			}
			if test.wantError && !strings.Contains(err.Error(), "force is not a downgrade flag") {
				t.Fatalf("downgrade error = %v", err)
			}
		})
	}
}

func TestExecuteCoreDevelopmentBuildPrecedesNetwork(t *testing.T) {
	client := &fakeReleaseClient{}
	result, err := executeCore(context.Background(), CoreOptions{Force: true}, coreDependencies{
		installedVersion: "dev",
		releases:         client,
	})
	if err != nil || !result.DevelopmentBuild || client.metadataCalls != 0 || len(client.downloads) != 0 {
		t.Fatalf("result=%#v err=%v metadata=%d downloads=%v", result, err, client.metadataCalls, client.downloads)
	}
}

func TestExecuteCoreRejectsUnclassifiableVersionBeforeInspectionOrDownload(t *testing.T) {
	client := &fakeReleaseClient{release: &github.SelectedCLIRelease{Version: "v1.2.3", TagName: "v1.2.3"}}
	counter := &countingInspector{}
	result, err := executeCore(context.Background(), CoreOptions{}, coreDependencies{
		installedVersion: "unknown",
		releases:         client,
		inspector:        counter,
	})
	if err == nil || result.Action != ActionUnsupported || !strings.Contains(err.Error(), "cannot safely classify update") {
		t.Fatalf("result=%#v error=%v", result, err)
	}
	if counter.calls != 0 || len(client.downloads) != 0 {
		t.Fatalf("inspections=%d downloads=%v", counter.calls, client.downloads)
	}
}

func TestExecuteCorePermissionAndManagerRefusalPrecedeDownloads(t *testing.T) {
	for _, test := range []struct { //nolint:govet // Field order mirrors the refusal scenarios.
		name           string
		classification installationClassification
		manager        string
		parentAllowed  bool
		wantText       string
	}{
		{name: "privileged", classification: installationUnmanagedPrivileged, parentAllowed: false, wantText: "atomic replacement requires create, rename, and remove"},
		{name: "manager", classification: installationManagerOwned, manager: "Homebrew", parentAllowed: true, wantText: "brew upgrade neko-cli"},
	} {
		t.Run(test.name, func(t *testing.T) {
			deps, client, target, oldContent := coreFixture(t, "1.0.0", "1.1.0")
			deps.inspector = staticInspector{installation: installation{
				runningExecutable:    target,
				canonicalTarget:      target,
				targetParent:         filepath.Dir(target),
				targetMode:           0o755,
				targetReadable:       true,
				parentCreateAllowed:  test.parentAllowed,
				parentReplaceAllowed: test.parentAllowed,
				classification:       test.classification,
				manager:              test.manager,
				managerGuidance:      "brew upgrade neko-cli (or brew reinstall neko-cli)",
			}}
			counter := &countingReplacement{}
			deps.replacement = counter
			_, err := executeCore(context.Background(), CoreOptions{}, deps)
			if err == nil || !strings.Contains(err.Error(), test.wantText) || !strings.Contains(err.Error(), "no archive was downloaded") {
				t.Fatalf("error = %v", err)
			}
			if len(client.downloads) != 0 || counter.calls != 0 {
				t.Fatalf("downloads=%v reserve calls=%d", client.downloads, counter.calls)
			}
			content, readErr := os.ReadFile(target)
			if readErr != nil || string(content) != string(oldContent) {
				t.Fatalf("target=%q err=%v", content, readErr)
			}
		})
	}
}

func TestExecuteCoreDryRunNeverReservesOrDownloads(t *testing.T) {
	for _, test := range []struct { //nolint:govet // Field order mirrors the dry-run scenarios.
		name      string
		installed string
		available string
		force     bool
		action    Action
	}{
		{name: "upgrade", installed: "1.0.0", available: "1.1.0", action: ActionUpgrade},
		{name: "forced reinstall", installed: "1.1.0", available: "1.1.0", force: true, action: ActionForcedReinstall},
	} {
		t.Run(test.name, func(t *testing.T) {
			deps, client, _, _ := coreFixture(t, test.installed, test.available)
			counter := &countingReplacement{}
			deps.replacement = counter
			result, err := executeCore(context.Background(), CoreOptions{Force: test.force, DryRun: true}, deps)
			if err != nil || !result.DryRun || result.Action != test.action {
				t.Fatalf("result=%#v err=%v", result, err)
			}
			if counter.calls != 0 || len(client.downloads) != 0 {
				t.Fatalf("force=%t reserve=%d downloads=%v", test.force, counter.calls, client.downloads)
			}
		})
	}
}

func TestExecuteCoreForceCannotBypassChecksumVerification(t *testing.T) {
	deps, client, target, oldContent := coreFixture(t, "1.1.0", "1.1.0")
	client.bodies["checksums"] = []byte(fmt.Sprintf("%x  neko-cli_Darwin_arm64.tar.gz\n", sha256.Sum256([]byte("wrong"))))
	result, err := executeCore(context.Background(), CoreOptions{Force: true}, deps)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") || result.DestinationChanged {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	content, readErr := os.ReadFile(target)
	if readErr != nil || string(content) != string(oldContent) {
		t.Fatalf("target=%q err=%v", content, readErr)
	}
}

func TestExecuteCoreForceCannotBypassPrivilegedInstallationPolicy(t *testing.T) {
	deps, client, target, oldContent := coreFixture(t, "1.1.0", "1.1.0")
	deps.inspector = staticInspector{installation: installation{
		runningExecutable:    target,
		canonicalTarget:      target,
		targetParent:         filepath.Dir(target),
		classification:       installationUnmanagedPrivileged,
		targetMode:           0o755,
		targetOwnerUID:       0,
		targetOwnerGID:       0,
		ownerKnown:           true,
		targetReadable:       true,
		parentCreateAllowed:  false,
		parentReplaceAllowed: false,
	}}
	result, err := executeCore(context.Background(), CoreOptions{Force: true}, deps)
	if err == nil || result.Action != ActionForcedReinstall || !strings.Contains(err.Error(), "owner uid 0 gid 0") {
		t.Fatalf("result=%#v error=%v", result, err)
	}
	if len(client.downloads) != 0 {
		t.Fatalf("permission refusal downloads=%v", client.downloads)
	}
	content, readErr := os.ReadFile(target)
	if readErr != nil || string(content) != string(oldContent) {
		t.Fatalf("target=%q err=%v", content, readErr)
	}
}

func TestExecuteCoreUsesReservationAsAuthoritativeUnknownCapabilityCheck(t *testing.T) {
	deps, client, target, oldContent := coreFixture(t, "1.0.0", "1.1.0")
	deps.inspector = staticInspector{installation: installation{
		runningExecutable:    target,
		canonicalTarget:      target,
		targetParent:         filepath.Dir(target),
		classification:       installationUnknown,
		targetMode:           0o755,
		targetReadable:       true,
		parentCreateAllowed:  false,
		parentReplaceAllowed: false,
	}}
	deps.replacement = &osReplacementCapability{ops: &memoryReplacementOps{createErr: errors.New("exclusive sibling reservation denied")}}
	_, err := executeCore(context.Background(), CoreOptions{}, deps)
	if err == nil || !strings.Contains(err.Error(), "exclusive sibling reservation denied") {
		t.Fatalf("error=%v", err)
	}
	if len(client.downloads) != 0 {
		t.Fatalf("reservation refusal downloads=%v", client.downloads)
	}
	content, readErr := os.ReadFile(target)
	if readErr != nil || string(content) != string(oldContent) {
		t.Fatalf("target=%q err=%v", content, readErr)
	}
}

func TestExecuteCoreReportsCleanupFailureWithoutChangingTarget(t *testing.T) {
	deps, client, target, oldContent := coreFixture(t, "1.0.0", "1.1.0")
	client.bodies["checksums"] = []byte(fmt.Sprintf("%x  neko-cli_Darwin_arm64.tar.gz\n", sha256.Sum256([]byte("wrong"))))
	file := &faultReplacementFile{name: filepath.Join(filepath.Dir(target), ".neko-update-cleanup")}
	ops := &memoryReplacementOps{file: file, removeErr: errors.New("frozen cleanup failure")}
	deps.replacement = &osReplacementCapability{ops: ops}
	_, err := executeCore(context.Background(), CoreOptions{}, deps)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") || !strings.Contains(err.Error(), "frozen cleanup failure") {
		t.Fatalf("error = %v", err)
	}
	content, readErr := os.ReadFile(target)
	if readErr != nil || string(content) != string(oldContent) {
		t.Fatalf("target=%q err=%v", content, readErr)
	}
}

func TestExecuteCorePrecommitFailuresLeaveTargetByteIdentical(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*fakeReleaseClient)
		wantText  string
	}{
		{name: "checksum download", configure: func(client *fakeReleaseClient) {
			client.downloadErrors["checksums"] = errors.New("frozen checksum download failure")
		}, wantText: "frozen checksum download failure"},
		{name: "archive download", configure: func(client *fakeReleaseClient) {
			client.downloadErrors["archive"] = errors.New("frozen archive download failure")
		}, wantText: "frozen archive download failure"},
		{name: "checksum mismatch", configure: func(client *fakeReleaseClient) {
			client.bodies["checksums"] = []byte(fmt.Sprintf("%x  neko-cli_Darwin_arm64.tar.gz\n", sha256.Sum256([]byte("wrong"))))
		}, wantText: "checksum mismatch"},
		{name: "malformed archive", configure: func(client *fakeReleaseClient) {
			client.bodies["archive"] = []byte("not gzip")
			digest := sha256.Sum256(client.bodies["archive"])
			client.bodies["checksums"] = []byte(fmt.Sprintf("%x  neko-cli_Darwin_arm64.tar.gz\n", digest))
		}, wantText: "not valid gzip"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deps, client, target, oldContent := coreFixture(t, "1.0.0", "1.1.0")
			test.configure(client)
			_, err := executeCore(context.Background(), CoreOptions{}, deps)
			if err == nil || !strings.Contains(err.Error(), test.wantText) {
				t.Fatalf("error = %v", err)
			}
			content, readErr := os.ReadFile(target)
			if readErr != nil || string(content) != string(oldContent) {
				t.Fatalf("target changed to %q err=%v", content, readErr)
			}
			matches, globErr := filepath.Glob(filepath.Join(filepath.Dir(target), ".neko-update-*"))
			if globErr != nil || len(matches) != 0 {
				t.Fatalf("stale reservations=%v err=%v", matches, globErr)
			}
		})
	}
}

func TestExecuteCoreSupportsWritableSymlinkTarget(t *testing.T) {
	deps, client, target, _ := coreFixture(t, "1.0.0", "1.1.0")
	linkDir := t.TempDir()
	link := filepath.Join(linkDir, "neko")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	inspector := newOSInstallationInspector()
	inspector.executable = func() (string, error) { return link, nil }
	inspector.managerPrefixes = nil
	deps.inspector = inspector
	result, err := executeCore(context.Background(), CoreOptions{}, deps)
	if err != nil || !result.DestinationChanged || len(client.downloads) != 2 {
		t.Fatalf("result=%#v err=%v downloads=%v", result, err, client.downloads)
	}
	if got, err := os.Readlink(link); err != nil || got != target {
		t.Fatalf("symlink target=%q err=%v", got, err)
	}
}

func TestExecuteCoreRefusesNonWritableSymlinkBeforeDownload(t *testing.T) {
	deps, client, target, oldContent := coreFixture(t, "1.0.0", "1.1.0")
	link := filepath.Join(t.TempDir(), "neko")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	deps.inspector = staticInspector{installation: installation{
		runningExecutable:    link,
		symlinkPath:          link,
		canonicalTarget:      target,
		targetParent:         filepath.Dir(target),
		classification:       installationUnmanagedPrivileged,
		targetMode:           0o755,
		targetReadable:       true,
		parentCreateAllowed:  false,
		parentReplaceAllowed: false,
	}}
	_, err := executeCore(context.Background(), CoreOptions{}, deps)
	if err == nil || !strings.Contains(err.Error(), "invoked through symlink") || len(client.downloads) != 0 {
		t.Fatalf("error=%v downloads=%v", err, client.downloads)
	}
	content, readErr := os.ReadFile(target)
	linkedTarget, linkErr := os.Readlink(link)
	if readErr != nil || linkErr != nil || string(content) != string(oldContent) || linkedTarget != target {
		t.Fatalf("content=%q readErr=%v link=%q linkErr=%v", content, readErr, linkedTarget, linkErr)
	}
}

func TestExecuteCoreRefusesPositiveHomebrewSymlinkBeforeDownload(t *testing.T) {
	deps, client, _, _ := coreFixture(t, "1.0.0", "1.1.0")
	prefix := filepath.Join(t.TempDir(), "homebrew")
	target := filepath.Join(prefix, "Cellar", "neko-cli", "1.1.0", "bin", "neko")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	oldContent := []byte("manager-owned executable")
	if err := os.WriteFile(target, oldContent, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(prefix, "bin", "neko")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	inspector := newOSInstallationInspector()
	inspector.executable = func() (string, error) { return link, nil }
	inspector.managerPrefixes = []string{prefix}
	deps.inspector = inspector
	counter := &countingReplacement{}
	deps.replacement = counter
	_, err := executeCore(context.Background(), CoreOptions{}, deps)
	if err == nil || !strings.Contains(err.Error(), "managed by Homebrew") || !strings.Contains(err.Error(), "invoked through symlink") {
		t.Fatalf("error=%v", err)
	}
	if len(client.downloads) != 0 || counter.calls != 0 {
		t.Fatalf("downloads=%v reservations=%d", client.downloads, counter.calls)
	}
	content, readErr := os.ReadFile(target)
	linkedTarget, linkErr := os.Readlink(link)
	if readErr != nil || linkErr != nil || string(content) != string(oldContent) || linkedTarget != target {
		t.Fatalf("content=%q readErr=%v link=%q linkErr=%v", content, readErr, linkedTarget, linkErr)
	}
}

func TestExecuteCoreRejectsPlatformAndMissingAssetsWithoutDownload(t *testing.T) {
	deps, client, _, _ := coreFixture(t, "1.0.0", "1.1.0")
	deps.platform = platform{OS: "windows", Arch: "amd64"}
	if _, err := executeCore(context.Background(), CoreOptions{}, deps); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unsupported error = %v", err)
	}
	if len(client.downloads) != 0 {
		t.Fatalf("unsupported downloads = %v", client.downloads)
	}

	deps, client, _, _ = coreFixture(t, "1.0.0", "1.1.0")
	client.release.Assets = nil
	if _, err := executeCore(context.Background(), CoreOptions{}, deps); err == nil || !strings.Contains(err.Error(), "no compatible archive") {
		t.Fatalf("missing asset error = %v", err)
	}
	if len(client.downloads) != 0 {
		t.Fatalf("missing asset downloads = %v", client.downloads)
	}
}

func coreFixture(t *testing.T, installed, available string) (coreDependencies, *fakeReleaseClient, string, []byte) {
	t.Helper()
	root := t.TempDir()
	target := filepath.Join(root, "neko")
	oldContent := []byte("old executable")
	if err := os.WriteFile(target, oldContent, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := archiveFixture(t, tarFixture{name: "neko-cli", body: []byte("new executable"), typeFlag: 0})
	digest := sha256.Sum256(archive)
	client := &fakeReleaseClient{
		release: &github.SelectedCLIRelease{
			Version: normalizeVersion(available),
			TagName: normalizeVersion(available),
			Assets: []github.Asset{
				{Name: "neko-cli_Darwin_arm64.tar.gz", BrowserDownloadURL: "archive"},
				{Name: "neko-cli_" + strings.TrimPrefix(available, "v") + "_checksums.txt", BrowserDownloadURL: "checksums"},
			},
		},
		bodies: map[string][]byte{
			"archive":   archive,
			"checksums": []byte(fmt.Sprintf("%x  neko-cli_Darwin_arm64.tar.gz\n", digest)),
		},
		downloadErrors: make(map[string]error),
	}
	inspector := staticInspector{installation: installation{
		runningExecutable:    target,
		canonicalTarget:      target,
		targetParent:         root,
		targetMode:           0o755,
		targetReadable:       true,
		parentCreateAllowed:  true,
		parentReplaceAllowed: true,
		classification:       installationUnmanagedUser,
	}}
	return coreDependencies{
		installedVersion: installed,
		releases:         client,
		inspector:        inspector,
		replacement:      newOSReplacementCapability(),
		platform:         platform{OS: "darwin", Arch: "arm64"},
	}, client, target, oldContent
}

type fakeReleaseClient struct {
	release        *github.SelectedCLIRelease
	metadataErr    error
	bodies         map[string][]byte
	downloadErrors map[string]error
	downloads      []string
	repositories   []*github.RepoInfo
	metadataCalls  int
}

func (client *fakeReleaseClient) ResolveLatestCLIRelease(_ context.Context, repoInfo *github.RepoInfo) (*github.SelectedCLIRelease, error) {
	client.metadataCalls++
	client.repositories = append(client.repositories, repoInfo)
	if client.metadataErr != nil {
		return nil, client.metadataErr
	}
	return client.release, nil
}

func (client *fakeReleaseClient) Download(_ context.Context, url string, _ int64) ([]byte, error) {
	client.downloads = append(client.downloads, url)
	if err := client.downloadErrors[url]; err != nil {
		return nil, err
	}
	return append([]byte(nil), client.bodies[url]...), nil
}

type staticInspector struct {
	err          error
	installation installation
}

func (inspector staticInspector) Inspect() (installation, error) {
	return inspector.installation, inspector.err
}

type countingReplacement struct {
	calls int
}

type countingInspector struct {
	calls int
}

func (inspector *countingInspector) Inspect() (installation, error) {
	inspector.calls++
	return installation{}, errors.New("unexpected inspection")
}

func (replacement *countingReplacement) Reserve(installation) (*replacementReservation, error) {
	replacement.calls++
	return nil, errors.New("unexpected reservation")
}
