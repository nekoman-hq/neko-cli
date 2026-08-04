package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	github "github.com/nekoman-hq/neko-cli/pkg/git"
)

// documentedInstallCommand is the primary copy-ready installation command in the
// README. TestREADMEDocumentsTheTestedPipeInstallCommand requires the README to
// publish exactly this command, and the piped installer tests execute the same
// stdin-fed shape.
const documentedInstallCommand = "curl -fsSL https://raw.githubusercontent.com/nekoman-hq/neko-cli/main/install.sh | bash"

func TestInstallScriptHasNoLocalReleaseConfigDependency(t *testing.T) {
	script, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	for _, forbidden := range []string{
		".release.neko.json",
		".plugin.release.neko.json",
		".neko/release.state.json",
		".neko/release.config.json",
		"/releases/latest",
	} {
		if bytes.Contains(script, []byte(forbidden)) {
			t.Fatalf("install.sh must not contain %q", forbidden)
		}
	}
}

// TestInstallScriptDoesNotDependOnBASHSOURCE pins the stdin-execution fix.
// `curl ... | bash` leaves BASH_SOURCE empty, and reading BASH_SOURCE[0] under
// `set -u` aborted the installer with "BASH_SOURCE[0]: unbound variable".
func TestInstallScriptDoesNotDependOnBASHSOURCE(t *testing.T) {
	script, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	if bytes.Contains(script, []byte("BASH_SOURCE")) {
		t.Fatal("install.sh must not depend on BASH_SOURCE; stdin execution leaves it unset")
	}
}

func TestCLIReleaseDefinesAuthoritativeChecksums(t *testing.T) {
	config, err := os.ReadFile(".goreleaser.cli.yaml")
	if err != nil {
		t.Fatalf("read CLI GoReleaser config: %v", err)
	}
	for _, required := range []string{
		"checksum:",
		`name_template: "neko-cli_{{ if index .Env \"CLI_VERSION\" }}{{ .Env.CLI_VERSION }}{{ else }}{{ .Version }}{{ end }}_checksums.txt"`,
	} {
		if !bytes.Contains(config, []byte(required)) {
			t.Fatalf("CLI GoReleaser config missing %q", required)
		}
	}
}

// TestInstallScriptRunsIdenticallyFromAFileAndFromStdin covers the four
// documented invocation forms. Direct execution and stdin execution reach the
// same main flow and install the same binary.
func TestInstallScriptRunsIdenticallyFromAFileAndFromStdin(t *testing.T) {
	for _, mode := range []executionMode{executeFromFile, executeFromPipe, executeFromPipeStrict} {
		t.Run(string(mode), func(t *testing.T) {
			fixture := newInstallScriptFixture(t)
			fixture.writeMixedReleaseList()
			fixture.writeFakeCurl()

			output, err := fixture.runAs(mode, map[string]string{
				"NEKO_GITHUB_API_BASE": "https://api.example",
				"NEKO_INSTALL_DIR":     fixture.installDir,
				"NEKO_OS":              "Darwin",
				"NEKO_ARCH":            "arm64",
				"PATH":                 fixture.pathWith(),
			})
			if err != nil {
				t.Fatalf("install.sh (%s) failed: %v\n%s", mode, err, output)
			}
			if strings.Contains(output, "BASH_SOURCE") || strings.Contains(output, "unbound variable") {
				t.Fatalf("install.sh (%s) reported an unbound variable:\n%s", mode, output)
			}
			fixture.assertInstalled(fixture.installDir)
			if !strings.Contains(output, "neko-cli v3.1.2 installed to") {
				t.Fatalf("install.sh (%s) output:\n%s", mode, output)
			}
		})
	}
}

// TestInstallScriptResolvesLatestStableCLIRelease covers stable CLI selection
// from a mixed multi-unit release list and exact platform asset selection.
func TestInstallScriptResolvesLatestStableCLIRelease(t *testing.T) {
	fixture := newInstallScriptFixture(t)
	fixture.writeMixedReleaseList()
	fixture.writeFakeCurl()

	output, err := fixture.run(map[string]string{
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_INSTALL_DIR":     fixture.installDir,
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "arm64",
		"PATH":                 fixture.pathWith(),
	})
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}
	fixture.assertInstalled(fixture.installDir)

	log := fixture.curlLog()
	for _, expected := range []string{
		"https://api.example/repos/nekoman-hq/neko-cli/releases?per_page=100&page=1",
		"https://api.example/repos/nekoman-hq/neko-cli/releases/tags/v3.1.2",
		"https://download.example/neko-cli_Darwin_arm64.tar.gz",
	} {
		if !strings.Contains(log, expected) {
			t.Fatalf("curl log missing %q\nlog:\n%s", expected, log)
		}
	}
	for _, forbidden := range []string{
		"/releases/latest",
		"/releases/tags/plugin-release/v4.4.5",
		"plugin-release_Darwin_arm64.tar.gz",
		"plugin-ui_Darwin_arm64.tar.gz",
		"plugin-index.json",
	} {
		if strings.Contains(log, forbidden) {
			t.Fatalf("install.sh requested %q\nlog:\n%s", forbidden, log)
		}
	}
}

func TestInstallScriptSelectsThePlatformAsset(t *testing.T) {
	for _, test := range []struct {
		os    string
		arch  string
		asset string
	}{
		{os: "Darwin", arch: "arm64", asset: "neko-cli_Darwin_arm64.tar.gz"},
		{os: "Linux", arch: "amd64", asset: "neko-cli_Linux_x86_64.tar.gz"},
		{os: "Linux", arch: "aarch64", asset: "neko-cli_Linux_arm64.tar.gz"},
	} {
		t.Run(test.os+"/"+test.arch, func(t *testing.T) {
			fixture := newInstallScriptFixture(t)
			fixture.writeTagRelease("v3.1.2", []string{
				"plugin-release_Darwin_arm64.tar.gz",
				"neko-cli_Darwin_arm64.tar.gz",
				"neko-cli_Linux_x86_64.tar.gz",
				"neko-cli_Linux_arm64.tar.gz",
			})
			fixture.writeArchive(test.asset)
			fixture.writeFakeCurl()

			output, err := fixture.run(map[string]string{
				"NEKO_GITHUB_API_BASE": "https://api.example",
				"NEKO_INSTALL_DIR":     fixture.installDir,
				"NEKO_VERSION":         "v3.1.2",
				"NEKO_OS":              test.os,
				"NEKO_ARCH":            test.arch,
				"PATH":                 fixture.pathWith(),
			})
			if err != nil {
				t.Fatalf("install.sh failed: %v\n%s", err, output)
			}
			fixture.assertInstalled(fixture.installDir)
			if !strings.Contains(fixture.curlLog(), "https://download.example/"+test.asset) {
				t.Fatalf("curl log missing %q\nlog:\n%s", test.asset, fixture.curlLog())
			}
		})
	}
}

func TestInstallScriptReportsAMissingPlatformAsset(t *testing.T) {
	fixture := newInstallScriptFixture(t)
	fixture.writeTagRelease("v3.1.2", []string{"neko-cli_Linux_x86_64.tar.gz"})
	fixture.writeFakeCurl()

	output, err := fixture.run(map[string]string{
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_INSTALL_DIR":     fixture.installDir,
		"NEKO_VERSION":         "v3.1.2",
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "arm64",
		"PATH":                 fixture.pathWith(),
	})
	if err == nil {
		t.Fatalf("install.sh unexpectedly succeeded:\n%s", output)
	}
	if !strings.Contains(output, "asset neko-cli_Darwin_arm64.tar.gz not found in release v3.1.2") {
		t.Fatalf("expected a missing asset error, got:\n%s", output)
	}
	fixture.assertNotInstalled(fixture.installDir)
}

func TestInstallScriptReportsAPIFailure(t *testing.T) {
	fixture := newInstallScriptFixture(t)
	fixture.writeMixedReleaseList()
	fixture.writeFakeCurl()

	output, err := fixture.run(map[string]string{
		"FAKE_CURL_MODE":       "api-failure",
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_INSTALL_DIR":     fixture.installDir,
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "arm64",
		"PATH":                 fixture.pathWith(),
	})
	if err == nil {
		t.Fatalf("install.sh unexpectedly succeeded:\n%s", output)
	}
	if !strings.Contains(output, "unable to determine the latest CLI release") ||
		!strings.Contains(output, "failed to fetch releases for nekoman-hq/neko-cli") {
		t.Fatalf("expected an actionable discovery error, got:\n%s", output)
	}
	fixture.assertNotInstalled(fixture.installDir)
}

func TestInstallScriptReportsMalformedJSON(t *testing.T) {
	fixture := newInstallScriptFixture(t)
	fixture.writeMixedReleaseList()
	fixture.writeFakeCurl()

	output, err := fixture.run(map[string]string{
		"FAKE_CURL_MODE":       "malformed",
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_INSTALL_DIR":     fixture.installDir,
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "arm64",
		"PATH":                 fixture.pathWith(),
	})
	if err == nil {
		t.Fatalf("install.sh unexpectedly succeeded:\n%s", output)
	}
	if !strings.Contains(output, "unable to determine the latest CLI release") ||
		!strings.Contains(output, "malformed release response for nekoman-hq/neko-cli") {
		t.Fatalf("expected a malformed response error, got:\n%s", output)
	}
	fixture.assertNotInstalled(fixture.installDir)
}

func TestInstallScriptReportsNoStableCLIRelease(t *testing.T) {
	fixture := newInstallScriptFixture(t)
	fixture.writeReleasesPage(1, `[
		{"tag_name":"plugin-release/v4.4.5","draft":false,"prerelease":false},
		{"tag_name":"plugin-ui/v1.3.0","draft":false,"prerelease":false},
		{"tag_name":"plugin-registry","draft":false,"prerelease":false},
		{"tag_name":"v3.2.0","draft":true,"prerelease":false},
		{"tag_name":"v3.1.3-rc.1","draft":false,"prerelease":true},
		{"tag_name":"3.1.9","draft":false,"prerelease":false},
		{"tag_name":"v3.9","draft":false,"prerelease":false}
	]`)
	fixture.writeFakeCurl()

	output, err := fixture.run(map[string]string{
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_INSTALL_DIR":     fixture.installDir,
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "arm64",
		"PATH":                 fixture.pathWith(),
	})
	if err == nil {
		t.Fatalf("install.sh unexpectedly succeeded:\n%s", output)
	}
	if !strings.Contains(output, "publishes no stable CLI release matching vX.Y.Z") {
		t.Fatalf("expected a no-stable-release error, got:\n%s", output)
	}
	fixture.assertNotInstalled(fixture.installDir)
}

// TestInstallScriptFollowsEveryReleasePage proves the installer does not assume
// the repository has fewer than one page of releases.
func TestInstallScriptFollowsEveryReleasePage(t *testing.T) {
	firstPage := make([]map[string]any, 0, 100)
	for index := 0; index < 100; index++ {
		firstPage = append(firstPage, map[string]any{
			"tag_name": "plugin-release/v4.4." + itoa(index), "draft": false, "prerelease": false,
		})
	}
	fixture := newInstallScriptFixture(t)
	fixture.writeReleasesPage(1, mustMarshal(t, firstPage))
	fixture.writeReleasesPage(2, `[{"tag_name":"v3.1.2","draft":false,"prerelease":false}]`)
	fixture.writeTagRelease("v3.1.2", []string{"neko-cli_Darwin_arm64.tar.gz"})
	fixture.writeArchive("neko-cli_Darwin_arm64.tar.gz")
	fixture.writeFakeCurl()

	output, err := fixture.run(map[string]string{
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_INSTALL_DIR":     fixture.installDir,
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "arm64",
		"PATH":                 fixture.pathWith(),
	})
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}
	fixture.assertInstalled(fixture.installDir)

	log := fixture.curlLog()
	for _, expected := range []string{"per_page=100&page=1", "per_page=100&page=2"} {
		if !strings.Contains(log, expected) {
			t.Fatalf("curl log missing %q\nlog:\n%s", expected, log)
		}
	}
	if strings.Contains(log, "per_page=100&page=3") {
		t.Fatalf("install.sh kept paginating past a short page\nlog:\n%s", log)
	}
}

// installScriptMaxReleasePages mirrors MAX_RELEASE_PAGES in install.sh. The Go
// resolver uses the same limit through pkg/git.
const installScriptMaxReleasePages = 20

// TestInstallScriptRefusesATruncatedReleaseList pins the pagination boundary:
// when the last permitted page is still full the list did not end, so the
// installer must refuse rather than install the greatest tag it happened to see.
func TestInstallScriptRefusesATruncatedReleaseList(t *testing.T) {
	fullPage := make([]map[string]any, 0, 100)
	fullPage = append(fullPage, map[string]any{"tag_name": "v3.1.2", "draft": false, "prerelease": false})
	for index := 1; index < 100; index++ {
		fullPage = append(fullPage, map[string]any{
			"tag_name": "plugin-release/v4.4." + itoa(index), "draft": false, "prerelease": false,
		})
	}

	fixture := newInstallScriptFixture(t)
	fixture.writeFixture("releases-any-page.json", mustMarshal(t, fullPage))
	fixture.writeTagRelease("v3.1.2", []string{"neko-cli_Darwin_arm64.tar.gz"})
	fixture.writeArchive("neko-cli_Darwin_arm64.tar.gz")
	fixture.writeFakeCurl()

	output, err := fixture.run(map[string]string{
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_INSTALL_DIR":     fixture.installDir,
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "arm64",
		"PATH":                 fixture.pathWith(),
	})
	if err == nil {
		t.Fatalf("install.sh installed from a truncated release list:\n%s", output)
	}
	for _, fragment := range []string{
		"unable to determine the latest CLI release",
		"has more than " + itoa(installScriptMaxReleasePages) + " release pages",
		"the list was truncated",
		"pin a version with NEKO_VERSION",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected fragment %q in:\n%s", fragment, output)
		}
	}
	fixture.assertNotInstalled(fixture.installDir)

	log := fixture.curlLog()
	if strings.Contains(log, "/releases/tags/") {
		t.Fatalf("install.sh selected a release from a truncated list:\n%s", log)
	}
	if count := strings.Count(log, "releases?per_page=100&page="); count != installScriptMaxReleasePages {
		t.Fatalf("install.sh read %d page(s), want exactly the %d-page limit\nlog:\n%s", count, installScriptMaxReleasePages, log)
	}
	if !strings.Contains(log, "page="+itoa(installScriptMaxReleasePages)) {
		t.Fatalf("install.sh did not reach the page limit\nlog:\n%s", log)
	}
	if strings.Contains(log, "page="+itoa(installScriptMaxReleasePages+1)) {
		t.Fatalf("install.sh paged past the limit\nlog:\n%s", log)
	}
}

// TestInstallScriptAcceptsAFullPageFollowedByAShortPage keeps the truncation
// guard from firing on a list that merely fills its intermediate pages.
func TestInstallScriptAcceptsAFullPageFollowedByAShortPage(t *testing.T) {
	fullPage := make([]map[string]any, 0, 100)
	for index := 0; index < 100; index++ {
		fullPage = append(fullPage, map[string]any{
			"tag_name": "plugin-release/v4.4." + itoa(index), "draft": false, "prerelease": false,
		})
	}

	fixture := newInstallScriptFixture(t)
	for page := 1; page < installScriptMaxReleasePages; page++ {
		fixture.writeReleasesPage(page, mustMarshal(t, fullPage))
	}
	fixture.writeReleasesPage(installScriptMaxReleasePages, `[{"tag_name":"v3.1.2","draft":false,"prerelease":false}]`)
	fixture.writeTagRelease("v3.1.2", []string{"neko-cli_Darwin_arm64.tar.gz"})
	fixture.writeArchive("neko-cli_Darwin_arm64.tar.gz")
	fixture.writeFakeCurl()

	output, err := fixture.run(map[string]string{
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_INSTALL_DIR":     fixture.installDir,
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "arm64",
		"PATH":                 fixture.pathWith(),
	})
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}
	fixture.assertInstalled(fixture.installDir)
	if !strings.Contains(output, "neko-cli v3.1.2 installed to") {
		t.Fatalf("install.sh output:\n%s", output)
	}
}

func TestInstallScriptNormalizesExplicitVersionAndSkipsLatestLookup(t *testing.T) {
	for _, requested := range []string{"3.1.2", "v3.1.2"} {
		t.Run(requested, func(t *testing.T) {
			fixture := newInstallScriptFixture(t)
			fixture.writeTagRelease("v3.1.2", []string{"neko-cli_Linux_x86_64.tar.gz"})
			fixture.writeArchive("neko-cli_Linux_x86_64.tar.gz")
			fixture.writeFakeCurl()

			output, err := fixture.run(map[string]string{
				"NEKO_GITHUB_API_BASE": "https://api.example",
				"NEKO_INSTALL_DIR":     fixture.installDir,
				"NEKO_VERSION":         requested,
				"NEKO_OS":              "Linux",
				"NEKO_ARCH":            "amd64",
				"PATH":                 fixture.pathWith(),
			})
			if err != nil {
				t.Fatalf("install.sh failed: %v\n%s", err, output)
			}

			log := fixture.curlLog()
			if strings.Contains(log, "releases?per_page=") {
				t.Fatalf("explicit NEKO_VERSION must skip latest lookup\nlog:\n%s", log)
			}
			if !strings.Contains(log, "https://api.example/repos/nekoman-hq/neko-cli/releases/tags/v3.1.2") {
				t.Fatalf("curl log missing normalized tag lookup\nlog:\n%s", log)
			}
		})
	}
}

// TestInstallScriptHonorsAPipedNEKOVERSIONAndInstallDir covers the documented
// `curl ... | NEKO_VERSION=v3.1.2 bash` and `curl ... | NEKO_INSTALL_DIR=... bash`
// forms, where the environment is applied to the stdin-fed shell.
func TestInstallScriptHonorsAPipedNEKOVERSIONAndInstallDir(t *testing.T) {
	fixture := newInstallScriptFixture(t)
	userLocalBin := filepath.Join(fixture.tempDir, "home", ".local", "bin")
	fixture.writeTagRelease("v3.1.2", []string{"neko-cli_Darwin_arm64.tar.gz"})
	fixture.writeArchive("neko-cli_Darwin_arm64.tar.gz")
	fixture.writeFakeCurl()

	output, err := fixture.runAs(executeFromPipe, map[string]string{
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_VERSION":         "v3.1.2",
		"NEKO_INSTALL_DIR":     userLocalBin,
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "arm64",
		"PATH":                 fixture.pathWith(),
	})
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}
	fixture.assertInstalled(userLocalBin)
	if strings.Contains(fixture.curlLog(), "releases?per_page=") {
		t.Fatalf("piped NEKO_VERSION must skip latest lookup\nlog:\n%s", fixture.curlLog())
	}
	if !strings.Contains(output, "Add "+userLocalBin+" to PATH") {
		t.Fatalf("missing PATH guidance:\n%s", output)
	}
}

func TestInstallScriptDefaultsOrdinaryUsersToHomeLocalBin(t *testing.T) {
	fixture := newInstallScriptFixture(t)
	fixture.writeTagRelease("v3.1.2", []string{"neko-cli_Darwin_arm64.tar.gz"})
	fixture.writeArchive("neko-cli_Darwin_arm64.tar.gz")
	fixture.writeFakeCurl()
	fixture.writeFakeID("501")
	home := filepath.Join(fixture.tempDir, "home with spaces")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}

	output, err := fixture.run(map[string]string{
		"HOME":                 home,
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_VERSION":         "v3.1.2",
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "arm64",
		"PATH":                 fixture.pathWith(),
	})
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}
	installDir := filepath.Join(home, ".local", "bin")
	fixture.assertInstalled(installDir)
	if !strings.Contains(output, "Add "+installDir+" to PATH") {
		t.Fatalf("missing PATH guidance:\n%s", output)
	}
}

func TestInstallScriptPreservesExplicitPathWithSpacesAndSuppressesReachedPATHGuidance(t *testing.T) {
	fixture := newInstallScriptFixture(t)
	fixture.installDir = filepath.Join(fixture.tempDir, "explicit install with spaces")
	fixture.writeTagRelease("v3.1.2", []string{"neko-cli_Darwin_arm64.tar.gz"})
	fixture.writeArchive("neko-cli_Darwin_arm64.tar.gz")
	fixture.writeFakeCurl()
	fixture.writeFakeID("501")

	output, err := fixture.run(map[string]string{
		"HOME":                 fixture.tempDir,
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_INSTALL_DIR":     fixture.installDir,
		"NEKO_VERSION":         "v3.1.2",
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "arm64",
		"PATH":                 fixture.pathWith(fixture.installDir),
	})
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}
	fixture.assertInstalled(fixture.installDir)
	if strings.Contains(output, "to PATH") {
		t.Fatalf("unexpected PATH guidance:\n%s", output)
	}
}

func TestInstallScriptRootDefaultAndNoAutomaticSudoPolicy(t *testing.T) {
	fixture := newInstallScriptFixture(t)
	fixture.writeFakeID("0")
	fixture.writeFakeSudo()
	fixture.writeFailingMkdir()

	output, err := fixture.run(map[string]string{
		"HOME":                 fixture.tempDir,
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_VERSION":         "v3.1.2",
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "arm64",
		"PATH":                 fixture.pathWith(),
	})
	if err == nil {
		t.Fatalf("install.sh unexpectedly succeeded:\n%s", output)
	}
	if !strings.Contains(output, "cannot create installation directory: /usr/local/bin") {
		t.Fatalf("root default target = %q", output)
	}
	if log := fixture.mkdirLog(); !strings.Contains(log, "-p /usr/local/bin") {
		t.Fatalf("mkdir log = %q", log)
	}

	fixture = newInstallScriptFixture(t)
	fixture.writeFakeID("501")
	fixture.writeFakeSudo()
	if mkdirErr := os.MkdirAll(fixture.installDir, 0o755); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if chmodErr := os.Chmod(fixture.installDir, 0o555); chmodErr != nil {
		t.Fatal(chmodErr)
	}
	t.Cleanup(func() { _ = os.Chmod(fixture.installDir, 0o755) })
	installOutput, err := fixture.run(map[string]string{
		"HOME":             fixture.tempDir,
		"NEKO_INSTALL_DIR": fixture.installDir,
		"NEKO_VERSION":     "v3.1.2",
		"NEKO_OS":          "Darwin",
		"NEKO_ARCH":        "arm64",
		"PATH":             fixture.pathWith(),
	})
	if err == nil || !strings.Contains(installOutput, "never invokes sudo") {
		t.Fatalf("non-writable explicit target result: err=%v\n%s", err, installOutput)
	}
	if _, statErr := os.Stat(fixture.sudoLog); !os.IsNotExist(statErr) {
		t.Fatalf("sudo was invoked or marker stat failed: %v", statErr)
	}
	if log := fixture.curlLog(); log != "" {
		t.Fatalf("network fake invoked before target refusal:\n%s", log)
	}
}

func TestInstallScriptRejectsUnsupportedPlatform(t *testing.T) {
	fixture := newInstallScriptFixture(t)

	output, err := fixture.run(map[string]string{
		"NEKO_INSTALL_DIR": fixture.installDir,
		"NEKO_VERSION":     "v3.1.2",
		"NEKO_OS":          "Plan9",
		"NEKO_ARCH":        "arm64",
	})
	if err == nil {
		t.Fatalf("install.sh unexpectedly succeeded:\n%s", output)
	}
	if !strings.Contains(output, "unsupported operating system") {
		t.Fatalf("expected unsupported OS error, got:\n%s", output)
	}
}

func TestInstallScriptRejectsUnsupportedDarwinI386Target(t *testing.T) {
	fixture := newInstallScriptFixture(t)
	fixture.writeFakeCurl()

	output, err := fixture.run(map[string]string{
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_INSTALL_DIR":     fixture.installDir,
		"NEKO_VERSION":         "v3.1.2",
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "i386",
		"PATH":                 fixture.pathWith(),
	})
	if err == nil {
		t.Fatalf("install.sh unexpectedly accepted Darwin/i386:\n%s", output)
	}
	if !strings.Contains(output, "unsupported platform: Darwin/i386") {
		t.Fatalf("expected unsupported Darwin/i386 error, got:\n%s", output)
	}
}

func TestInstallScriptHonorsNEKORepositoryOverride(t *testing.T) {
	fixture := newInstallScriptFixture(t)
	fixture.writeTagRelease("v3.1.2", []string{"neko-cli_Darwin_arm64.tar.gz"})
	fixture.writeArchive("neko-cli_Darwin_arm64.tar.gz")
	fixture.writeFakeCurl()

	output, err := fixture.run(map[string]string{
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_REPOSITORY":      "forkowner/forkrepo",
		"NEKO_INSTALL_DIR":     fixture.installDir,
		"NEKO_VERSION":         "v3.1.2",
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "arm64",
		"PATH":                 fixture.pathWith(),
	})
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}
	if !strings.Contains(fixture.curlLog(), "https://api.example/repos/forkowner/forkrepo/releases/tags/v3.1.2") {
		t.Fatalf("curl log did not use the configured repository:\n%s", fixture.curlLog())
	}
}

// TestBashAndGoCLIReleaseResolversSelectTheSameStableTag is the cross-contract
// guard. Both the Bash resolver in install.sh and the Go resolver in pkg/git run
// over the same checked-in release fixtures and must select the same tag, so the
// installer and the built-in updater cannot drift apart again.
func TestBashAndGoCLIReleaseResolversSelectTheSameStableTag(t *testing.T) {
	for _, testCase := range loadCLIReleaseContract(t) {
		t.Run(testCase.Name, func(t *testing.T) {
			releases := readContractFixture(t, testCase.Releases)

			var goReleases []github.Release
			if err := json.Unmarshal(releases, &goReleases); err != nil {
				t.Fatalf("decode %s: %v", testCase.Releases, err)
			}
			goSelected, err := github.SelectStableCLIRelease(goReleases)
			if err != nil {
				t.Fatalf("Go resolver: %v", err)
			}
			if goSelected.TagName != testCase.ExpectedTag {
				t.Fatalf("Go resolver selected %q, want %q", goSelected.TagName, testCase.ExpectedTag)
			}

			fixture := newInstallScriptFixture(t)
			fixture.writeReleasesPage(1, string(releases))
			fixture.writeTagReleaseFromList(releases, testCase.ExpectedTag)
			fixture.writeArchive("neko-cli_Darwin_arm64.tar.gz")
			fixture.writeFakeCurl()

			output, runErr := fixture.run(map[string]string{
				"NEKO_GITHUB_API_BASE": "https://api.example",
				"NEKO_INSTALL_DIR":     fixture.installDir,
				"NEKO_OS":              "Darwin",
				"NEKO_ARCH":            "arm64",
				"PATH":                 fixture.pathWith(),
			})
			if runErr != nil {
				t.Fatalf("install.sh failed: %v\n%s", runErr, output)
			}
			if !strings.Contains(output, "neko-cli "+testCase.ExpectedTag+" installed to") {
				t.Fatalf("Bash resolver output:\n%s", output)
			}
			if !strings.Contains(fixture.curlLog(), "/releases/tags/"+testCase.ExpectedTag) {
				t.Fatalf("Bash resolver requested:\n%s", fixture.curlLog())
			}
			if goSelected.TagName != testCase.ExpectedTag {
				t.Fatalf("resolver drift: Go %q vs Bash %q", goSelected.TagName, testCase.ExpectedTag)
			}
		})
	}
}

func TestREADMEDocumentsTheTestedPipeInstallCommand(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	document := string(readme)

	if !strings.Contains(document, documentedInstallCommand) {
		t.Fatalf("README must document the primary command %q", documentedInstallCommand)
	}
	if !strings.HasSuffix(documentedInstallCommand, "| bash") {
		t.Fatalf("the documented primary command %q is not a pipe invocation", documentedInstallCommand)
	}
	if !strings.Contains(documentedInstallCommand, "/install.sh") {
		t.Fatalf("the documented primary command %q does not fetch install.sh", documentedInstallCommand)
	}

	for _, required := range []string{
		"curl -fsSL https://raw.githubusercontent.com/nekoman-hq/neko-cli/main/install.sh | NEKO_VERSION=vX.Y.Z bash",
		`curl -fsSL https://raw.githubusercontent.com/nekoman-hq/neko-cli/main/install.sh | NEKO_INSTALL_DIR="$HOME/.local/bin" bash`,
		"curl -fsSL https://raw.githubusercontent.com/nekoman-hq/neko-cli/main/install.sh -o /tmp/neko-install.sh",
		"bash /tmp/neko-install.sh",
		"rm /tmp/neko-install.sh",
		"neko plugin install",
		"It installs only the main `neko` CLI. It never installs a plugin.",
		"whose tag matches exactly `vX.Y.Z`",
		"mutable `plugin-registry` release, drafts, prereleases, and malformed tags are",
		"must be present in `PATH`",
		"/usr/local/bin",
		"$HOME/.local/bin",
	} {
		if !strings.Contains(document, required) {
			t.Fatalf("README must document %q", required)
		}
	}
}

// TestREADMEUsesAPlaceholderForTheSpecificVersionExample keeps the documented
// specific-version form in step with the repository documentation contract,
// which forbids pinning a current release version in the README.
func TestREADMEUsesAPlaceholderForTheSpecificVersionExample(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	state, err := os.ReadFile(filepath.Join(".neko", "release.state.json"))
	if err != nil {
		t.Fatalf("read release state: %v", err)
	}

	var releaseState struct {
		Units map[string]struct {
			Version string `json:"version"`
		} `json:"units"`
	}
	if err := json.Unmarshal(state, &releaseState); err != nil {
		t.Fatalf("decode release state: %v", err)
	}
	for unit, entry := range releaseState.Units {
		if entry.Version == "" {
			continue
		}
		if strings.Contains(string(readme), entry.Version) {
			t.Fatalf("README pins the current %s version %q; use the vX.Y.Z placeholder", unit, entry.Version)
		}
	}
}

type cliReleaseContractCase struct {
	Name            string `json:"name"`
	Releases        string `json:"releases"`
	ExpectedTag     string `json:"expected_tag"`
	ExpectedVersion string `json:"expected_version"`
}

func loadCLIReleaseContract(t *testing.T) []cliReleaseContractCase {
	t.Helper()

	data := readContractFixture(t, "contract.json")
	var cases []cliReleaseContractCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("decode CLI release contract: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("CLI release contract is empty")
	}
	return cases
}

func readContractFixture(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "cli-release-contract", name))
	if err != nil {
		t.Fatalf("read contract fixture %s: %v", name, err)
	}
	return data
}

type executionMode string

const (
	executeFromFile       executionMode = "bash install.sh"
	executeFromPipe       executionMode = "cat install.sh | bash"
	executeFromPipeStrict executionMode = "cat install.sh | bash -u"
)

type installScriptFixture struct {
	t          *testing.T
	root       string
	tempDir    string
	fixtures   string
	binDir     string
	installDir string
	logPath    string
	sudoLog    string
	mkdirLogP  string
}

func newInstallScriptFixture(t *testing.T) *installScriptFixture {
	t.Helper()

	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmp := t.TempDir()
	return &installScriptFixture{
		t:          t,
		root:       root,
		tempDir:    tmp,
		fixtures:   filepath.Join(tmp, "fixtures"),
		binDir:     filepath.Join(tmp, "bin"),
		installDir: filepath.Join(tmp, "install"),
		logPath:    filepath.Join(tmp, "curl.log"),
		sudoLog:    filepath.Join(tmp, "sudo.log"),
		mkdirLogP:  filepath.Join(tmp, "mkdir.log"),
	}
}

func (f *installScriptFixture) pathWith(extra ...string) string {
	parts := append([]string{f.binDir}, extra...)
	parts = append(parts, os.Getenv("PATH"))
	return strings.Join(parts, string(os.PathListSeparator))
}

// writeMixedReleaseList installs the shared multi-unit release fixture together
// with the release document and archive for the stable CLI tag it selects.
func (f *installScriptFixture) writeMixedReleaseList() {
	f.t.Helper()

	releases := readContractFixture(f.t, "mixed-releases.json")
	f.writeReleasesPage(1, string(releases))
	f.writeTagReleaseFromList(releases, "v3.1.2")
	f.writeArchive("neko-cli_Darwin_arm64.tar.gz")
	f.writeArchive("neko-cli_Linux_x86_64.tar.gz")
}

func (f *installScriptFixture) writeReleasesPage(page int, body string) {
	f.t.Helper()
	f.writeFixture("releases-page-"+itoa(page)+".json", body)
}

func (f *installScriptFixture) writeTagRelease(tag string, assetNames []string) {
	f.t.Helper()

	assets := make([]map[string]string, 0, len(assetNames))
	for _, name := range assetNames {
		assets = append(assets, map[string]string{
			"name":                 name,
			"browser_download_url": "https://download.example/" + name,
		})
	}
	f.writeFixture("release-"+tag+".json", mustMarshal(f.t, map[string]any{
		"tag_name": tag,
		"assets":   assets,
	}))
}

// writeTagReleaseFromList derives the per-tag release document from the shared
// release list, so the list fixture and the tag lookup cannot disagree.
func (f *installScriptFixture) writeTagReleaseFromList(releases []byte, tag string) {
	f.t.Helper()

	var entries []map[string]any
	if err := json.Unmarshal(releases, &entries); err != nil {
		f.t.Fatalf("decode release list: %v", err)
	}
	for _, entry := range entries {
		if entry["tag_name"] == tag {
			f.writeFixture("release-"+tag+".json", mustMarshal(f.t, entry))
			return
		}
	}
	f.t.Fatalf("release list has no entry for tag %q", tag)
}

func (f *installScriptFixture) writeFixture(name, body string) {
	f.t.Helper()

	if err := os.MkdirAll(f.fixtures, 0o755); err != nil {
		f.t.Fatalf("create fixtures dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(f.fixtures, name), []byte(body), 0o644); err != nil {
		f.t.Fatalf("write %s: %v", name, err)
	}
}

func (f *installScriptFixture) writeFakeID(uid string) {
	f.t.Helper()
	f.writeFakeBinary("id", "#!/usr/bin/env bash\nif [ \"${1:-}\" = \"-u\" ]; then printf '%s\\n' \"${FAKE_ID_UID}\"; else /usr/bin/id \"$@\"; fi\n")
	f.t.Setenv("FAKE_ID_UID", uid)
}

func (f *installScriptFixture) writeFakeSudo() {
	f.t.Helper()
	f.writeFakeBinary("sudo", "#!/usr/bin/env bash\nprintf 'invoked\\n' >>\"${SUDO_LOG}\"\nexit 99\n")
}

// writeFailingMkdir records the requested directory and refuses to create it, so
// the root default destination is observable without writing to a real system
// directory and without sudo.
func (f *installScriptFixture) writeFailingMkdir() {
	f.t.Helper()
	f.writeFakeBinary("mkdir", "#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >>\"${MKDIR_LOG}\"\nexit 1\n")
}

func (f *installScriptFixture) writeFakeBinary(name, script string) {
	f.t.Helper()

	if err := os.MkdirAll(f.binDir, 0o755); err != nil {
		f.t.Fatalf("create bin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(f.binDir, name), []byte(script), 0o755); err != nil {
		f.t.Fatalf("write fake %s: %v", name, err)
	}
}

func (f *installScriptFixture) writeArchive(name string) {
	f.t.Helper()

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)
	content := []byte("#!/usr/bin/env sh\necho fake neko\n")
	header := &tar.Header{
		Name: "neko-cli",
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		f.t.Fatalf("write tar header: %v", err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		f.t.Fatalf("write tar body: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		f.t.Fatalf("close tar: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		f.t.Fatalf("close gzip: %v", err)
	}

	f.writeFixture(name, buf.String())
}

// writeFakeCurl serves the fixture directory over the same URL shapes the
// installer requests. It never contacts GitHub.
func (f *installScriptFixture) writeFakeCurl() {
	f.t.Helper()

	f.writeFakeBinary("curl", `#!/usr/bin/env bash
set -uo pipefail
url="${@: -1}"
echo "$url" >> "$CURL_LOG"
mode="${FAKE_CURL_MODE:-ok}"

case "$url" in
  */releases\?per_page=*)
    case "$mode" in
      api-failure)
        echo "curl: (22) simulated HTTP 500 for $url" >&2
        exit 22
        ;;
      malformed)
        printf '%s' '{"tag_name": "v3.1.2"'
        exit 0
        ;;
    esac
    page="${url##*page=}"
    file="$FIXTURE_DIR/releases-page-${page}.json"
    if [ -f "$file" ]; then
      cat "$file"
    elif [ -f "$FIXTURE_DIR/releases-any-page.json" ]; then
      cat "$FIXTURE_DIR/releases-any-page.json"
    else
      printf '%s' '[]'
    fi
    ;;
  */releases/tags/*)
    tag="${url##*/releases/tags/}"
    file="$FIXTURE_DIR/release-${tag}.json"
    if [ -f "$file" ]; then
      cat "$file"
    else
      echo "curl: (22) no release fixture for tag ${tag}" >&2
      exit 22
    fi
    ;;
  https://download.example/*)
    name="${url##*/}"
    file="$FIXTURE_DIR/${name}"
    if [ -f "$file" ]; then
      cat "$file"
    else
      echo "curl: (22) no asset fixture for ${name}" >&2
      exit 22
    fi
    ;;
  *)
    echo "unexpected curl URL: $url" >&2
    exit 22
    ;;
esac
`)
}

func (f *installScriptFixture) run(env map[string]string) (string, error) {
	f.t.Helper()
	return f.runAs(executeFromFile, env)
}

func (f *installScriptFixture) runAs(mode executionMode, env map[string]string) (string, error) {
	f.t.Helper()

	scriptPath := filepath.Join(f.root, "install.sh")
	var cmd *exec.Cmd
	switch mode {
	case executeFromFile:
		cmd = exec.Command("bash", scriptPath)
	case executeFromPipe:
		cmd = exec.Command("bash", "-c", `cat "$1" | bash`, "bash", scriptPath)
	case executeFromPipeStrict:
		// `bash -u` makes every unbound expansion fatal from the first line, which
		// is what `curl ... | bash` exposed through BASH_SOURCE[0].
		cmd = exec.Command("bash", "-c", `set -u; cat "$1" | bash -u`, "bash", scriptPath)
	default:
		f.t.Fatalf("unknown execution mode %q", mode)
	}
	cmd.Dir = f.root

	baseEnvironment := make([]string, 0, len(os.Environ()))
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "NEKO_") || strings.HasPrefix(value, "GITHUB_TOKEN=") || strings.HasPrefix(value, "HOME=") {
			continue
		}
		baseEnvironment = append(baseEnvironment, value)
	}
	cmd.Env = append(baseEnvironment,
		"CURL_LOG="+f.logPath,
		"FIXTURE_DIR="+f.fixtures,
		"SUDO_LOG="+f.sudoLog,
		"MKDIR_LOG="+f.mkdirLogP,
	)
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (f *installScriptFixture) assertInstalled(dir string) {
	f.t.Helper()

	info, err := os.Stat(filepath.Join(dir, "neko"))
	if err != nil {
		f.t.Fatalf("expected installed neko binary in %s: %v", dir, err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		f.t.Fatalf("installed neko binary is not executable: %v", info.Mode())
	}
}

func (f *installScriptFixture) assertNotInstalled(dir string) {
	f.t.Helper()

	if _, err := os.Stat(filepath.Join(dir, "neko")); !os.IsNotExist(err) {
		f.t.Fatalf("installer left a binary in %s: %v", dir, err)
	}
}

func (f *installScriptFixture) curlLog() string {
	return f.readLog(f.logPath)
}

func (f *installScriptFixture) mkdirLog() string {
	return f.readLog(f.mkdirLogP)
}

func (f *installScriptFixture) readLog(path string) string {
	f.t.Helper()

	log, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		f.t.Fatalf("read log %s: %v", path, err)
	}
	return string(log)
}

func mustMarshal(t *testing.T, value any) string {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return string(data)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 12)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
