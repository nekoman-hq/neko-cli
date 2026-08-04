package update

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
	"strings"
	"testing"

	github "github.com/nekoman-hq/neko-cli/pkg/git"
)

func assertUnchanged(t *testing.T, target string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("target %s changed", target)
	}
}

// TestExecuteCoreUpgradesFromTheReportedInstalledVersion reproduces the reported
// defect: installed 3.0.3 with a newest stable CLI release of v3.1.2 must plan
// an upgrade, not report the installed version as the latest.
func TestExecuteCoreUpgradesFromTheReportedInstalledVersion(t *testing.T) {
	deps, client, _, _ := coreFixture(t, "3.0.3", "3.1.2")
	result, err := executeCore(context.Background(), CoreOptions{DryRun: true}, deps)
	if err != nil {
		t.Fatalf("executeCore: %v", err)
	}
	if result.Action != ActionUpgrade {
		t.Fatalf("action = %q, want %q", result.Action, ActionUpgrade)
	}
	if result.InstalledVersion != "3.0.3" || result.SelectedVersion != "3.1.2" {
		t.Fatalf("installed=%q selected=%q", result.InstalledVersion, result.SelectedVersion)
	}
	if len(client.downloads) != 0 {
		t.Fatalf("dry-run downloaded %v", client.downloads)
	}
}

// TestExecuteCoreReportsDiscoveryFailureInsteadOfAlreadyLatest pins the failure
// contract: a lookup failure must surface, never degrade into the installed
// version being reported as current.
func TestExecuteCoreReportsDiscoveryFailureInsteadOfAlreadyLatest(t *testing.T) {
	for _, cause := range []error{
		stderrors.New("unable to determine the latest CLI release for nekoman-hq/neko-cli: GitHub API returned status 500"),
		stderrors.New("unable to determine the latest CLI release for nekoman-hq/neko-cli: JSON Parse Failed: unexpected end of JSON input"),
		github.ErrNoReleases,
	} {
		client := &fakeReleaseClient{metadataErr: cause}
		counter := &countingInspector{}
		result, err := executeCore(context.Background(), CoreOptions{}, coreDependencies{
			installedVersion: "3.0.3",
			releases:         client,
			inspector:        counter,
		})
		if err == nil {
			t.Fatalf("discovery failure %v produced no error", cause)
		}
		if result.Action == ActionAlreadyCurrent {
			t.Fatalf("discovery failure %v was reported as already current", cause)
		}
		if result.SelectedVersion != "" {
			t.Fatalf("discovery failure %v selected version %q", cause, result.SelectedVersion)
		}
		if !strings.Contains(err.Error(), cause.Error()) {
			t.Fatalf("error = %q, want the discovery cause %q", err.Error(), cause.Error())
		}
		if counter.calls != 0 || len(client.downloads) != 0 {
			t.Fatalf("discovery failure inspected %d time(s) and downloaded %v", counter.calls, client.downloads)
		}
	}
}

func TestExecuteCoreRefusesToDowngradeANewerLocalBuild(t *testing.T) {
	deps, client, _, _ := coreFixture(t, "3.2.0", "3.1.2")
	result, err := executeCore(context.Background(), CoreOptions{}, deps)
	if err != nil {
		t.Fatalf("executeCore: %v", err)
	}
	if result.Action != ActionInstalledNewer {
		t.Fatalf("action = %q, want %q", result.Action, ActionInstalledNewer)
	}
	if len(client.downloads) != 0 {
		t.Fatalf("newer local build downloaded %v", client.downloads)
	}
}

func TestExecuteCoreResolvesTheConfiguredCLIRepository(t *testing.T) {
	deps, client, _, _ := coreFixture(t, "3.0.3", "3.1.2")
	deps.repository = github.CLIRepository()
	if _, err := executeCore(context.Background(), CoreOptions{DryRun: true}, deps); err != nil {
		t.Fatalf("executeCore: %v", err)
	}
	if len(client.repositories) != 1 {
		t.Fatalf("release lookups = %d, want exactly one", len(client.repositories))
	}
	resolved := client.repositories[0]
	if resolved == nil || resolved.Owner != "nekoman-hq" || resolved.Repo != "neko-cli" {
		t.Fatalf("resolved repository = %+v, want the configured CLI repository", resolved)
	}
}

func TestExecuteCoreDefaultsToTheConfiguredRepositoryWhenUnset(t *testing.T) {
	deps, client, _, _ := coreFixture(t, "3.0.3", "3.1.2")
	deps.repository = nil
	if _, err := executeCore(context.Background(), CoreOptions{DryRun: true}, deps); err != nil {
		t.Fatalf("executeCore: %v", err)
	}
	if len(client.repositories) != 1 || client.repositories[0].Owner != "nekoman-hq" || client.repositories[0].Repo != "neko-cli" {
		t.Fatalf("resolved repositories = %+v", client.repositories)
	}
}

func TestExecuteCoreReportsAnActionableErrorForAMissingPlatformAsset(t *testing.T) {
	deps, client, target, oldContent := coreFixture(t, "3.0.3", "3.1.2")
	client.release.Assets = []github.Asset{{Name: "neko-cli_Linux_x86_64.tar.gz", BrowserDownloadURL: "archive"}}

	_, err := executeCore(context.Background(), CoreOptions{}, deps)
	if err == nil {
		t.Fatal("missing platform asset produced no error")
	}
	if !strings.Contains(err.Error(), "no compatible archive for darwin/arm64") {
		t.Fatalf("error = %q, want the platform named", err.Error())
	}
	if len(client.downloads) != 0 {
		t.Fatalf("missing asset downloaded %v", client.downloads)
	}
	assertUnchanged(t, target, oldContent)
}

func TestExecuteCoreReportsAlreadyCurrentOnlyWhenVersionsAreEqual(t *testing.T) {
	deps, _, _, _ := coreFixture(t, "3.1.2", "3.1.2")
	result, err := executeCore(context.Background(), CoreOptions{}, deps)
	if err != nil {
		t.Fatalf("executeCore: %v", err)
	}
	if result.Action != ActionAlreadyCurrent {
		t.Fatalf("action = %q, want %q", result.Action, ActionAlreadyCurrent)
	}
}
