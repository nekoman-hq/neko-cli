package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestInstallScriptResolvesLatestStableCLIRelease(t *testing.T) {
	fixture := newInstallScriptFixture(t)
	fixture.writeJSON("releases.json", `[
		{"tag_name":"plugin-release/v9.9.9","draft":false,"prerelease":false},
		{"tag_name":"plugin-registry","draft":false,"prerelease":false},
		{"tag_name":"v3.0.4","draft":false,"prerelease":false},
		{"tag_name":"v3.0.9","draft":false,"prerelease":true},
		{"tag_name":"v3.0.10","draft":false,"prerelease":false}
	]`)
	fixture.writeJSON("release-v3.0.10.json", `{
		"tag_name":"v3.0.10",
		"assets":[
			{"name":"plugin-release_Darwin_arm64.tar.gz","browser_download_url":"https://download.example/plugin-release_Darwin_arm64.tar.gz"},
			{"name":"neko-cli_Darwin_arm64.tar.gz","browser_download_url":"https://download.example/neko-cli_Darwin_arm64.tar.gz"}
		]
	}`)
	fixture.writeArchive("neko-cli_Darwin_arm64.tar.gz")
	fixture.writeFakeCurl()

	output, err := fixture.run(map[string]string{
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_INSTALL_DIR":     fixture.installDir,
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "arm64",
		"PATH":                 fixture.binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(fixture.installDir, "neko")); err != nil {
		t.Fatalf("expected installed neko binary: %v", err)
	}

	log := fixture.curlLog()
	for _, expected := range []string{
		"https://api.example/repos/nekoman-hq/neko-cli/releases?per_page=100",
		"https://api.example/repos/nekoman-hq/neko-cli/releases/tags/v3.0.10",
		"https://download.example/neko-cli_Darwin_arm64.tar.gz",
	} {
		if !strings.Contains(log, expected) {
			t.Fatalf("curl log missing %q\nlog:\n%s", expected, log)
		}
	}
	if strings.Contains(log, "/releases/latest") {
		t.Fatalf("install.sh must not call /releases/latest\nlog:\n%s", log)
	}
}

func TestInstallScriptNormalizesExplicitVersionAndSkipsLatestLookup(t *testing.T) {
	fixture := newInstallScriptFixture(t)
	fixture.writeJSON("release-v3.0.4.json", `{
		"tag_name":"v3.0.4",
		"assets":[
			{"name":"neko-cli_Linux_x86_64.tar.gz","browser_download_url":"https://download.example/neko-cli_Linux_x86_64.tar.gz"}
		]
	}`)
	fixture.writeArchive("neko-cli_Linux_x86_64.tar.gz")
	fixture.writeFakeCurl()

	output, err := fixture.run(map[string]string{
		"NEKO_GITHUB_API_BASE": "https://api.example",
		"NEKO_INSTALL_DIR":     fixture.installDir,
		"NEKO_VERSION":         "3.0.4",
		"NEKO_OS":              "Linux",
		"NEKO_ARCH":            "amd64",
		"PATH":                 fixture.binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}

	log := fixture.curlLog()
	if strings.Contains(log, "releases?per_page=100") {
		t.Fatalf("explicit NEKO_VERSION must skip latest lookup\nlog:\n%s", log)
	}
	if !strings.Contains(log, "https://api.example/repos/nekoman-hq/neko-cli/releases/tags/v3.0.4") {
		t.Fatalf("curl log missing normalized tag lookup\nlog:\n%s", log)
	}
}

func TestInstallScriptRejectsUnsupportedPlatform(t *testing.T) {
	fixture := newInstallScriptFixture(t)

	output, err := fixture.run(map[string]string{
		"NEKO_INSTALL_DIR": fixture.installDir,
		"NEKO_VERSION":     "v3.0.4",
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
		"NEKO_VERSION":         "v3.0.4",
		"NEKO_OS":              "Darwin",
		"NEKO_ARCH":            "i386",
		"PATH":                 fixture.binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	if err == nil {
		t.Fatalf("install.sh unexpectedly accepted Darwin/i386:\n%s", output)
	}
	if !strings.Contains(output, "unsupported platform: Darwin/i386") {
		t.Fatalf("expected unsupported Darwin/i386 error, got:\n%s", output)
	}
}

type installScriptFixture struct {
	t          *testing.T
	root       string
	fixtures   string
	binDir     string
	installDir string
	logPath    string
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
		fixtures:   filepath.Join(tmp, "fixtures"),
		binDir:     filepath.Join(tmp, "bin"),
		installDir: filepath.Join(tmp, "install"),
		logPath:    filepath.Join(tmp, "curl.log"),
	}
}

func (f *installScriptFixture) writeJSON(name, body string) {
	f.t.Helper()

	if err := os.MkdirAll(f.fixtures, 0o755); err != nil {
		f.t.Fatalf("create fixtures dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(f.fixtures, name), []byte(body), 0o644); err != nil {
		f.t.Fatalf("write %s: %v", name, err)
	}
}

func (f *installScriptFixture) writeArchive(name string) {
	f.t.Helper()

	if err := os.MkdirAll(f.fixtures, 0o755); err != nil {
		f.t.Fatalf("create fixtures dir: %v", err)
	}

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

	if err := os.WriteFile(filepath.Join(f.fixtures, name), buf.Bytes(), 0o644); err != nil {
		f.t.Fatalf("write archive %s: %v", name, err)
	}
}

func (f *installScriptFixture) writeFakeCurl() {
	f.t.Helper()

	if err := os.MkdirAll(f.binDir, 0o755); err != nil {
		f.t.Fatalf("create bin dir: %v", err)
	}

	script := `#!/usr/bin/env bash
set -euo pipefail
url="${@: -1}"
echo "$url" >> "$CURL_LOG"

case "$url" in
  */releases\?per_page=100)
    cat "$FIXTURE_DIR/releases.json"
    ;;
  */releases/tags/v3.0.10)
    cat "$FIXTURE_DIR/release-v3.0.10.json"
    ;;
  */releases/tags/v3.0.4)
    cat "$FIXTURE_DIR/release-v3.0.4.json"
    ;;
  https://download.example/*)
    cat "$FIXTURE_DIR/$(basename "$url")"
    ;;
  *)
    echo "unexpected curl URL: $url" >&2
    exit 22
    ;;
esac
`
	curlPath := filepath.Join(f.binDir, "curl")
	if err := os.WriteFile(curlPath, []byte(script), 0o755); err != nil {
		f.t.Fatalf("write fake curl: %v", err)
	}
}

func (f *installScriptFixture) run(env map[string]string) (string, error) {
	f.t.Helper()

	cmd := exec.Command("bash", filepath.Join(f.root, "install.sh"))
	cmd.Dir = f.root
	cmd.Env = append(os.Environ(),
		"CURL_LOG="+f.logPath,
		"FIXTURE_DIR="+f.fixtures,
	)
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (f *installScriptFixture) curlLog() string {
	f.t.Helper()

	log, err := os.ReadFile(f.logPath)
	if err != nil {
		f.t.Fatalf("read curl log: %v", err)
	}
	return string(log)
}
