package pluginindex

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginIndexScriptsAreShellParsable(t *testing.T) {
	root := repositoryRoot(t)
	for _, script := range []string{
		".github/scripts/generate-plugin-index.sh",
		".github/scripts/publish-plugin-index.sh",
	} {
		cmd := exec.Command("bash", "-n", filepath.Join(root, script))
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("bash -n %s failed: %v\n%s", script, err, output)
		}
	}
}

func TestPluginIndexScriptsFailClearlyWhenEnvMissing(t *testing.T) {
	root := repositoryRoot(t)

	cmd := exec.Command("bash", filepath.Join(root, ".github/scripts/generate-plugin-index.sh"))
	cmd.Dir = root
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("generate script succeeded without required env")
	}
	if !strings.Contains(string(output), "INDEX_OUTPUT is required") {
		t.Fatalf("generate script error = %q", output)
	}

	cmd = exec.Command("bash", filepath.Join(root, ".github/scripts/publish-plugin-index.sh"))
	cmd.Dir = root
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("publish script succeeded without required env")
	}
	if !strings.Contains(string(output), "GITHUB_REPOSITORY is required") {
		t.Fatalf("publish script error = %q", output)
	}
}

func TestGeneratePluginIndexScriptGeneratesAndValidatesIndex(t *testing.T) {
	root := repositoryRoot(t)
	installFakeGoAndJQ(t)
	outputPath := filepath.Join(newPluginIndexTempDir(t), "plugin-index.json")

	cmd := exec.Command("bash", filepath.Join(root, ".github/scripts/generate-plugin-index.sh"))
	cmd.Dir = root
	cmd.Env = generateScriptEnv(t, outputPath, "plugin-release", "4.0.2", "plugin-release/v4.0.2")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate script failed: %v\n%s", err, output)
	}

	index := readFile(t, outputPath)
	assertContains(t, index, `"repository":"nekoman-hq/neko-cli"`)
	assertContains(t, index, `"unit":"plugin-release"`)
	assertContains(t, index, `"version":"4.0.2"`)
	assertContains(t, index, `"tag":"plugin-release/v4.0.2"`)
	assertNotContains(t, string(output), "ghp_secret")
}

func TestGeneratePluginIndexScriptFailsOnReleaseEntryMismatch(t *testing.T) {
	root := repositoryRoot(t)
	installFakeGoAndJQ(t)
	outputPath := filepath.Join(newPluginIndexTempDir(t), "plugin-index.json")

	cmd := exec.Command("bash", filepath.Join(root, ".github/scripts/generate-plugin-index.sh"))
	cmd.Dir = root
	cmd.Env = append(generateScriptEnv(t, outputPath, "plugin-release", "4.0.2", "plugin-release/v4.0.2"),
		"GO_FAKE_INDEX_MODE=mismatch",
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("generate script succeeded with mismatched release entry")
	}
	if !strings.Contains(string(output), "plugin-index.json must contain plugin-release 4.0.2 at plugin-release/v4.0.2") {
		t.Fatalf("generate script mismatch error = %q", output)
	}
}

func TestPublishPluginIndexScriptUploadsWhenRegistryReleaseExists(t *testing.T) {
	root := repositoryRoot(t)
	logPath := installFakeGH(t, "0")
	indexPath := writeTempIndex(t)

	cmd := exec.Command("bash", filepath.Join(root, ".github/scripts/publish-plugin-index.sh"))
	cmd.Dir = root
	cmd.Env = publishScriptEnv(t, logPath, indexPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("publish script failed: %v\n%s", err, output)
	}
	if strings.Contains(string(output), "ghp_secret") {
		t.Fatalf("publish script leaked token in output: %s", output)
	}

	log := readFile(t, logPath)
	assertContains(t, log, "release view plugin-registry --repo nekoman-hq/neko-cli")
	assertContains(t, log, "release upload plugin-registry "+indexPath+"#plugin-index.json --clobber --repo nekoman-hq/neko-cli")
	assertNotContains(t, log, "release create plugin-registry")
	assertNotContains(t, log, "releases/latest")
}

func TestPublishPluginIndexScriptCreatesRegistryReleaseWhenMissing(t *testing.T) {
	root := repositoryRoot(t)
	logPath := installFakeGH(t, "1")
	indexPath := writeTempIndex(t)

	cmd := exec.Command("bash", filepath.Join(root, ".github/scripts/publish-plugin-index.sh"))
	cmd.Dir = root
	cmd.Env = publishScriptEnv(t, logPath, indexPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("publish script failed: %v\n%s", err, output)
	}

	log := readFile(t, logPath)
	assertContains(t, log, "release view plugin-registry --repo nekoman-hq/neko-cli")
	assertContains(t, log, "release create plugin-registry "+indexPath+"#plugin-index.json --repo nekoman-hq/neko-cli --title Neko Plugin Registry --notes Mutable registry release for neko plugin-index.json. --target 0123456789abcdef0123456789abcdef01234567")
	assertNotContains(t, log, "release upload plugin-registry")
	assertNotContains(t, log, "releases/latest")
}

func TestPluginReleaseWorkflowsPublishPluginIndexAfterPluginRelease(t *testing.T) {
	root := repositoryRoot(t)

	action := readFile(
		t,
		filepath.Join(root, ".github/actions/publish-plugin-index/action.yml"),
	)

	for _, want := range []string{
		"name: Publish Plugin Index",
		"runs:\n  using: composite",
		"- name: Generate Plugin Index",
		"- name: Publish Plugin Index",
		`"$GITHUB_WORKSPACE/.github/scripts/generate-plugin-index.sh"`,
		`"$GITHUB_WORKSPACE/.github/scripts/publish-plugin-index.sh"`,
		"INDEX_OUTPUT:",
		"GITHUB_REPOSITORY:",
		"RELEASE_UNIT:",
		"RELEASE_VERSION:",
		"RELEASE_TAG:",
		"PLUGIN_REGISTRY_TARGET_SHA:",
		`if [[ -z "${GH_TOKEN:-}" && -z "${GITHUB_TOKEN:-}" ]]; then`,
	} {
		assertContains(t, action, want)
	}

	assertNotContains(t, action, "github-token:")

	generateIndex := strings.Index(action, "- name: Generate Plugin Index")
	publishIndex := strings.Index(action, "- name: Publish Plugin Index")

	if generateIndex < 0 || publishIndex < 0 {
		t.Fatalf("plugin-index action is missing generation or publication steps")
	}

	if generateIndex >= publishIndex {
		t.Fatalf("plugin-index action must generate the index before publishing it")
	}

	tests := []struct {
		path        string
		publishStep string
	}{
		{
			path:        ".github/workflows/release-plugin-release.yml",
			publishStep: "Publish plugin-release GitHub Release",
		},
		{
			path:        ".github/workflows/release-plugin-ui.yml",
			publishStep: "Publish plugin-ui GitHub Release",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			workflow := readFile(t, filepath.Join(root, tt.path))

			for _, want := range []string{
				"permissions:\n  contents: read",
				"  publish:\n",
				"    permissions:\n      contents: write",
				tt.publishStep,
				"- name: Publish Plugin Index",
				"uses: ./.github/actions/publish-plugin-index",
				"GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}",
				"repository: ${{ github.repository }}",
				"unit: ${{ needs.validate.outputs.unit }}",
				"version: ${{ needs.validate.outputs.version }}",
				"tag: ${{ needs.validate.outputs.tag }}",
				"release-sha: ${{ needs.validate.outputs.release_sha }}",
				"plugin-index.json",
				"plugin-registry",
			} {
				assertContains(t, workflow, want)
			}

			assertNotContains(t, workflow, "releases/latest")
			assertNotContains(t, workflow, ".github/scripts/generate-plugin-index.sh")
			assertNotContains(t, workflow, ".github/scripts/publish-plugin-index.sh")
			assertNotContains(t, workflow, "github-token:")

			releaseIndex := strings.Index(workflow, tt.publishStep)
			actionIndex := strings.Index(
				workflow,
				"uses: ./.github/actions/publish-plugin-index",
			)

			if releaseIndex < 0 || actionIndex < 0 {
				t.Fatalf("workflow is missing release publication or plugin-index action")
			}

			if releaseIndex >= actionIndex {
				t.Fatalf("plugin-index action must run after plugin release publication")
			}
		})
	}
}

func TestPluginIndexScriptsContainRequiredSafetyChecks(t *testing.T) {
	root := repositoryRoot(t)
	generateScript := readFile(t, filepath.Join(root, ".github/scripts/generate-plugin-index.sh"))
	for _, want := range []string{
		"--output-file \"$INDEX_OUTPUT\"",
		".schemaVersion == 1",
		".repository == $repository",
		".unit == $unit",
		".version == $version",
		".tag == $tag",
		".tag == (.tagPrefix + .version)",
		"startswith(\"/\")",
		"GITHUB_TOKEN",
		"GH_TOKEN",
	} {
		assertContains(t, generateScript, want)
	}
	assertNotContains(t, generateScript, "plugin-index \\\n  --output \"$INDEX_OUTPUT\"")

	publishScript := readFile(t, filepath.Join(root, ".github/scripts/publish-plugin-index.sh"))
	for _, want := range []string{
		"gh release view \"$registry_tag\"",
		"gh release upload \"$registry_tag\" \"$INDEX_PATH#$asset_name\" --clobber",
		"gh release create \"$registry_tag\" \"$INDEX_PATH#$asset_name\"",
		"--target \"$target_sha\"",
	} {
		assertContains(t, publishScript, want)
	}
	assertNotContains(t, publishScript, "releases/latest")
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func installFakeGH(t *testing.T, viewExit string) string {
	t.Helper()
	dir := newPluginIndexTempDir(t)
	logPath := filepath.Join(dir, "gh.log")
	fakeGH := `#!/bin/sh
printf '%s\n' "$*" >> "$GH_FAKE_LOG"
if [ "$1" = "release" ] && [ "$2" = "view" ]; then
  exit "$GH_FAKE_VIEW_EXIT"
fi
exit 0
`
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(fakeGH), 0755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GH_FAKE_VIEW_EXIT", viewExit)
	return logPath
}

func installFakeGoAndJQ(t *testing.T) {
	t.Helper()
	dir := newPluginIndexTempDir(t)

	fakeGo := `#!/bin/sh
set -eu
if [ "$1" = "build" ]; then
  output=""
  shift
  while [ "$#" -gt 0 ]; do
    if [ "$1" = "-o" ]; then
      output="$2"
      shift 2
      continue
    fi
    shift
  done
  mkdir -p "$(dirname "$output")"
  printf '#!/bin/sh\n' > "$output"
  chmod +x "$output"
  exit 0
fi
if [ "$1" = "run" ]; then
  output=""
  repository=""
  while [ "$#" -gt 0 ]; do
    if [ "$1" = "--output-file" ]; then
      output="$2"
      shift 2
      continue
    fi
    if [ "$1" = "--repository" ]; then
      repository="$2"
      shift 2
      continue
    fi
    shift
  done
  tag="$RELEASE_TAG"
  if [ "${GO_FAKE_INDEX_MODE:-}" = "mismatch" ]; then
    tag="plugin-release/v9.9.9"
  fi
  if [ "$RELEASE_UNIT" = "plugin-ui" ]; then
    name="ui"
    tag_prefix="plugin-ui/v"
    asset_prefix="plugin-ui"
    binary_name="plugin-ui"
  else
    name="release"
    tag_prefix="plugin-release/v"
    asset_prefix="plugin-release"
    binary_name="plugin-release"
  fi
  printf '{"schemaVersion":1,"repository":"%s","plugins":[{"name":"%s","unit":"%s","version":"%s","tag":"%s","tagPrefix":"%s","assetPrefix":"%s","binaryName":"%s"}]}\n' "$repository" "$name" "$RELEASE_UNIT" "$RELEASE_VERSION" "$tag" "$tag_prefix" "$asset_prefix" "$binary_name" > "$output"
  exit 0
fi
echo "unexpected go invocation: $*" >&2
exit 1
`

	fakeJQ := `#!/bin/sh
set -eu
repository=""
unit=""
version=""
tag=""
query=""
file=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -e)
      shift
      ;;
    --arg)
      name="$2"
      value="$3"
      case "$name" in
        repository) repository="$value" ;;
        unit) unit="$value" ;;
        version) version="$value" ;;
        tag) tag="$value" ;;
      esac
      shift 3
      ;;
    *)
      if [ "$#" -eq 1 ]; then
        file="$1"
      else
        query="$1"
      fi
      shift
      ;;
  esac
done
if echo "$query" | grep -q 'startswith'; then
  if grep -Eq ':"/' "$file"; then
    exit 0
  fi
  exit 1
fi
grep -F '"schemaVersion":1' "$file" >/dev/null
grep -F "\"repository\":\"$repository\"" "$file" >/dev/null
grep -F "\"unit\":\"$unit\"" "$file" >/dev/null
grep -F "\"version\":\"$version\"" "$file" >/dev/null
grep -F "\"tag\":\"$tag\"" "$file" >/dev/null
exit 0
`

	writeExecutable(t, filepath.Join(dir, "go"), fakeGo)
	writeExecutable(t, filepath.Join(dir, "jq"), fakeJQ)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func generateScriptEnv(t *testing.T, outputPath, unit, version, tag string) []string {
	t.Helper()
	return append(os.Environ(),
		"INDEX_OUTPUT="+outputPath,
		"GITHUB_REPOSITORY=nekoman-hq/neko-cli",
		"RELEASE_UNIT="+unit,
		"RELEASE_VERSION="+version,
		"RELEASE_TAG="+tag,
		"RUNNER_TEMP="+newPluginIndexTempDir(t),
		"GITHUB_TOKEN=ghp_secret",
	)
}

func publishScriptEnv(t *testing.T, logPath, indexPath string) []string {
	t.Helper()
	return append(os.Environ(),
		"GH_FAKE_LOG="+logPath,
		"GH_FAKE_VIEW_EXIT="+os.Getenv("GH_FAKE_VIEW_EXIT"),
		"GH_TOKEN=ghp_secret",
		"GITHUB_REPOSITORY=nekoman-hq/neko-cli",
		"GITHUB_SHA=0123456789abcdef0123456789abcdef01234567",
		"INDEX_PATH="+indexPath,
	)
}

func writeTempIndex(t *testing.T) string {
	t.Helper()
	path := filepath.Join(newPluginIndexTempDir(t), "plugin-index.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"repository":"nekoman-hq/neko-cli","plugins":[]}`), 0644); err != nil {
		t.Fatalf("write temp index: %v", err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("did not expect %q in:\n%s", needle, haystack)
	}
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected %q in:\n%s", needle, haystack)
	}
}
