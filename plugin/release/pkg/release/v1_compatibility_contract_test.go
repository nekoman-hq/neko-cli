//nolint:staticcheck // These tests intentionally pin the deprecated V1 compatibility contract.
package release

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type v1RegistryTool struct {
	name string
}

func (tool *v1RegistryTool) Name() string                                       { return tool.name }
func (*v1RegistryTool) Init(*config.V1ReleaseConfig) error                      { return nil }
func (*v1RegistryTool) Execute(*ReleaseExecutionContext) error                  { return nil }
func (*v1RegistryTool) ValidateRequirements(*ReleaseExecutionContext) error     { return nil }
func (*v1RegistryTool) ResolveFiles(*ReleaseExecutionContext) ([]string, error) { return nil, nil }
func (*v1RegistryTool) Release(*semver.Version) error                           { return nil }
func (*v1RegistryTool) RevertRelease() error                                    { return nil }

func TestV1ToolRegistryCompatibility(t *testing.T) {
	original := tools
	tools = make(map[string]Tool)
	t.Cleanup(func() { tools = original })

	first := &v1RegistryTool{name: "legacy"}
	second := &v1RegistryTool{name: "legacy"}
	Register(first)
	Register(second)

	got, err := Get("legacy")
	if err != nil {
		t.Fatalf("Get registered tool: %v", err)
	}
	if got != second {
		t.Fatal("later V1 registration must replace the previous tool with the same name")
	}
	if _, err := Get("missing"); err == nil || err.Error() != "unknown release system: missing" {
		t.Fatalf("unknown registry error = %v", err)
	}
}

func TestV1VersionGuardRefreshContract(t *testing.T) {
	originalFetch := refreshVersionTags
	originalLatest := latestVersionTag
	t.Cleanup(func() {
		refreshVersionTags = originalFetch
		latestVersionTag = originalLatest
	})

	var events []string
	refreshVersionTags = func() { events = append(events, "fetch") }
	latestVersionTag = func() string {
		events = append(events, "latest-tag")
		return "1.2.3"
	}
	cfg := &config.V1ReleaseConfig{Version: "1.2.3"}

	if _, err := VersionGuardWithOptions(cfg, VersionGuardOptions{AllowRemoteRefresh: false}); err != nil {
		t.Fatalf("local VersionGuardWithOptions: %v", err)
	}
	if got := strings.Join(events, ","); got != "latest-tag" {
		t.Fatalf("local planning events = %q, want latest-tag", got)
	}

	events = nil
	if _, err := VersionGuard(cfg); err != nil {
		t.Fatalf("release VersionGuard: %v", err)
	}
	if got := strings.Join(events, ","); got != "fetch,latest-tag" {
		t.Fatalf("release planning events = %q, want fetch,latest-tag", got)
	}
}

func TestV1VersionGuardCompatibilityRules(t *testing.T) {
	tests := []struct {
		name      string
		local     string
		latest    string
		want      string
		wantError string
	}{
		{name: "equal", local: "1.2.3", latest: "v1.2.3", want: "1.2.3"},
		{name: "local ahead", local: "1.3.0", latest: "v1.2.3", want: "1.3.0"},
		{name: "unparseable latest falls back to local", local: "1.2.3", latest: "not-semver", want: "1.2.3"},
		{name: "local behind", local: "1.2.2", latest: "v1.2.3", wantError: "version violation: Local version 1.2.2 is smaller than latest tag 1.2.3"},
		{name: "invalid local", local: "bad", latest: "v1.2.3", wantError: "version bad in .release.neko.json is not a valid semantic version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EnsureVersionIsValid(&config.V1ReleaseConfig{Version: tt.local}, tt.latest)
			if tt.wantError != "" {
				if err == nil || err.Error() != tt.wantError {
					t.Fatalf("EnsureVersionIsValid error = %v, want %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("EnsureVersionIsValid: %v", err)
			}
			if got.String() != tt.want {
				t.Fatalf("version = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestV1FatalPreflightCompatibilityResponse(t *testing.T) {
	if os.Getenv("NEKO_TEST_V1_FATAL_PREFLIGHT") == "1" {
		Preflight(nil)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestV1FatalPreflightCompatibilityResponse$")
	cmd.Env = append(os.Environ(), "NEKO_TEST_V1_FATAL_PREFLIGHT=1")
	output, err := cmd.Output()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("fatal preflight exit = %v, want code 1; output=%s", err, output)
	}

	var response plugin.Response
	if err := json.Unmarshal(output, &response); err != nil {
		t.Fatalf("decode fatal response %q: %v", output, err)
	}
	if response.Status != "error" || response.Error == nil || response.Error.Code != "RELEASE_REQUIREMENTS_INVALID" {
		t.Fatalf("unexpected fatal preflight response: %#v", response)
	}
	if response.Error.Message != "release configuration is missing" {
		t.Fatalf("fatal preflight message = %q", response.Error.Message)
	}
	if response.Metadata.Command != "" {
		t.Fatalf("fatal compatibility response unexpectedly set command %q", response.Metadata.Command)
	}
}

func TestV1PreflightFatalCodeMatrix(t *testing.T) {
	if scenario := os.Getenv("NEKO_TEST_V1_PREFLIGHT_SCENARIO"); scenario != "" {
		Preflight(validV1ReleaseConfig("1.2.3"))
		return
	}

	tests := []struct {
		scenario string
		code     string
	}{
		{scenario: "dirty", code: "UNCOMMITTED_CHANGES"},
		{scenario: "detached", code: "DETACHED_HEAD"},
		{scenario: "feature", code: "INCORRECT_BRANCH"},
		{scenario: "no-upstream", code: "NO_UPSTREAM_BRANCH"},
		{scenario: "behind", code: "BRANCH_OUT_OF_DATE"},
	}
	for _, tt := range tests {
		t.Run(tt.scenario, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, goReleaserConfigFileYML), []byte("{}"), 0644); err != nil {
				t.Fatalf("write GoReleaser config: %v", err)
			}
			binDir := t.TempDir()
			writeV1PreflightScenarioGit(t, filepath.Join(binDir, "git"))

			cmd := exec.Command(os.Args[0], "-test.run=^TestV1PreflightFatalCodeMatrix$")
			cmd.Dir = root
			cmd.Env = append(os.Environ(),
				"NEKO_TEST_V1_PREFLIGHT_SCENARIO="+tt.scenario,
				"GITHUB_TOKEN=test-token",
				"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
			)
			output, err := cmd.Output()
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
				t.Fatalf("preflight exit = %v, want code 1; output=%s", err, output)
			}
			var response plugin.Response
			if err := json.Unmarshal(output, &response); err != nil {
				t.Fatalf("decode fatal response %q: %v", output, err)
			}
			if response.Error == nil || response.Error.Code != tt.code {
				t.Fatalf("fatal code = %#v, want %s", response.Error, tt.code)
			}
		})
	}
}

type v1ObservingTool struct {
	executeError   error
	configVersion  string
	contextVersion string
	reverted       bool
}

func (*v1ObservingTool) Name() string                                            { return "goreleaser" }
func (*v1ObservingTool) Init(*config.V1ReleaseConfig) error                      { return nil }
func (*v1ObservingTool) ValidateRequirements(*ReleaseExecutionContext) error     { return nil }
func (*v1ObservingTool) ResolveFiles(*ReleaseExecutionContext) ([]string, error) { return nil, nil }
func (*v1ObservingTool) Release(*semver.Version) error                           { return nil }
func (tool *v1ObservingTool) Execute(ctx *ReleaseExecutionContext) error {
	data, err := os.ReadFile(config.V1FileName)
	if err != nil {
		return fmt.Errorf("observe V1 config: %w", err)
	}
	var cfg config.V1ReleaseConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("decode observed V1 config: %w", err)
	}
	tool.configVersion = cfg.Version
	tool.contextVersion = ctx.NextVersion
	return tool.executeError
}
func (tool *v1ObservingTool) RevertRelease() error {
	tool.reverted = true
	return nil
}

func TestV1ReleaseCompatibilityUsesLocalPreviewThenRefreshedExecution(t *testing.T) {
	withWorkingDirectory(t)
	installV1PreflightGit(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	if err := os.WriteFile(goReleaserConfigFileYML, []byte("{}"), 0644); err != nil {
		t.Fatalf("write GoReleaser config: %v", err)
	}
	cfg := validV1ReleaseConfig("1.2.3")
	if err := config.V1SaveConfig(*cfg); err != nil {
		t.Fatalf("write V1 config: %v", err)
	}

	tool := &v1ObservingTool{}
	installV1RegistryTool(t, tool)
	latestCalls, fetchCalls := installV1VersionEvidence(t, "v1.2.3")
	ctx := v1CompatibilityExecutionContext(t, Patch, false)

	outcome, failure := startLegacyRelease(ctx, cfg)
	if failure != nil {
		t.Fatalf("startLegacyRelease failure: %#v", failure)
	}
	completed, ok := outcome.(*LegacyReleaseCompleted)
	if !ok {
		t.Fatalf("outcome = %T", outcome)
	}
	if completed.PreviousVersion != "1.2.3" || completed.NextVersion != "1.2.4" {
		t.Fatalf("unexpected completion versions: %#v", completed)
	}
	if *latestCalls != 2 || *fetchCalls != 1 {
		t.Fatalf("version evidence calls: latest=%d fetch=%d, want latest=2 fetch=1", *latestCalls, *fetchCalls)
	}
	if tool.configVersion != "1.2.4" || tool.contextVersion != "1.2.4" {
		t.Fatalf("executor observed config=%q context=%q, want 1.2.4", tool.configVersion, tool.contextVersion)
	}
}

func TestV1ReleaseCompatibilityRestoresConfigThroughCompensationBoundary(t *testing.T) {
	withWorkingDirectory(t)
	installV1PreflightGit(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	if err := os.WriteFile(goReleaserConfigFileYML, []byte("{}"), 0644); err != nil {
		t.Fatalf("write GoReleaser config: %v", err)
	}
	cfg := validV1ReleaseConfig("1.2.3")
	if err := config.V1SaveConfig(*cfg); err != nil {
		t.Fatalf("write V1 config: %v", err)
	}

	tool := &v1ObservingTool{executeError: errors.New("publish failed")}
	installV1RegistryTool(t, tool)
	installV1VersionEvidence(t, "v1.2.3")
	_, failure := startLegacyRelease(v1CompatibilityExecutionContext(t, Patch, false), cfg)
	if failure == nil || failure.Code != "RELEASE_FAILED" || !strings.Contains(failure.responseMessage(), "publish failed") {
		t.Fatalf("failure = %#v", failure)
	}
	if tool.reverted {
		t.Fatal("active V1 compensation delegated back to the legacy rollback wrapper")
	}
	restored, err := config.V1LoadConfig()
	if err != nil {
		t.Fatalf("load restored V1 config: %v", err)
	}
	if restored.Version != "1.2.3" {
		t.Fatalf("restored V1 version = %q, want 1.2.3", restored.Version)
	}
}

func TestV1ReleaseCompatibilityStopsBeforeExecutorWhenOriginalConfigCannotBeCaptured(t *testing.T) {
	withWorkingDirectory(t)
	installV1PreflightGit(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	if err := os.WriteFile(goReleaserConfigFileYML, []byte("{}"), 0644); err != nil {
		t.Fatalf("write GoReleaser config: %v", err)
	}
	if err := os.Mkdir(config.V1FileName, 0755); err != nil {
		t.Fatalf("create unwritable V1 target shape: %v", err)
	}

	tool := &v1ObservingTool{}
	// This variant observes only the context because the V1 path is deliberately
	// a directory and therefore cannot be read as a config file.
	toolWithoutConfigRead := &v1ContextOnlyTool{v1ObservingTool: tool}
	installV1RegistryTool(t, toolWithoutConfigRead)
	installV1VersionEvidence(t, "v1.2.3")
	cfg := validV1ReleaseConfig("1.2.3")
	service := NewReleaseServiceWithContext(cfg, v1CompatibilityExecutionContext(t, Patch, false))

	if err := service.Run(Patch); err == nil || !strings.Contains(err.Error(), "read original V1 config") {
		t.Fatalf("Service.Run error = %v", err)
	}
	if cfg.Version != "1.2.3" || tool.contextVersion != "" {
		t.Fatalf("unsafe release continued: config=%q context=%q", cfg.Version, tool.contextVersion)
	}
}

type v1ContextOnlyTool struct {
	*v1ObservingTool
}

func (tool *v1ContextOnlyTool) Execute(ctx *ReleaseExecutionContext) error {
	tool.contextVersion = ctx.NextVersion
	return tool.executeError
}

func validV1ReleaseConfig(version string) *config.V1ReleaseConfig {
	return &config.V1ReleaseConfig{
		ProjectName:   "example",
		ProjectOwner:  "acme",
		ProjectType:   config.V1ProjectTypeBackend,
		ReleaseSystem: config.V1ReleaseTypeGoReleaser,
		Version:       version,
	}
}

func installV1RegistryTool(t *testing.T, tool Tool) {
	t.Helper()
	original := tools
	tools = map[string]Tool{tool.Name(): tool}
	t.Cleanup(func() { tools = original })
}

func installV1VersionEvidence(t *testing.T, latest string) (*int, *int) {
	t.Helper()
	originalLatest := latestVersionTag
	originalFetch := refreshVersionTags
	latestCalls := 0
	fetchCalls := 0
	latestVersionTag = func() string {
		latestCalls++
		return latest
	}
	refreshVersionTags = func() { fetchCalls++ }
	t.Cleanup(func() {
		latestVersionTag = originalLatest
		refreshVersionTags = originalFetch
	})
	return &latestCalls, &fetchCalls
}

func v1CompatibilityExecutionContext(t *testing.T, releaseType Type, dryRun bool) *ReleaseExecutionContext {
	t.Helper()
	tagSpec, err := config.NewTagSpec("v")
	if err != nil {
		t.Fatalf("NewTagSpec: %v", err)
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	return &ReleaseExecutionContext{
		RepositoryRoot: root,
		UnitRoot:       root,
		ReleaseKind:    releaseType,
		DryRun:         dryRun,
		Executor:       "goreleaser",
		SourceFormat:   config.SourceFormatV1,
		TagSpec:        tagSpec,
	}
}

func installV1PreflightGit(t *testing.T) {
	t.Helper()
	binDir := t.TempDir()
	script := `#!/bin/sh
case "$*" in
	"rev-parse --git-common-dir") printf '.git\n' ;;
	"remote -v")
    printf 'origin\thttps://github.com/acme/example.git (fetch)\norigin\thttps://github.com/acme/example.git (push)\n'
    ;;
  "status --porcelain") ;;
  "rev-parse --abbrev-ref HEAD") printf 'main\n' ;;
  "for-each-ref --format=%(upstream:short) refs/heads/main") printf 'origin/main\n' ;;
  "status -sb") printf '## main...origin/main\n' ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0755); err != nil {
		t.Fatalf("write fake preflight git: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func writeV1PreflightScenarioGit(t *testing.T, path string) {
	t.Helper()
	script := `#!/bin/sh
case "$*" in
  "status --porcelain")
    if [ "$NEKO_TEST_V1_PREFLIGHT_SCENARIO" = "dirty" ]; then printf ' M file\n'; fi
    ;;
  "rev-parse --abbrev-ref HEAD")
    case "$NEKO_TEST_V1_PREFLIGHT_SCENARIO" in
      detached) printf 'HEAD\n' ;;
      feature) printf 'feature\n' ;;
      *) printf 'main\n' ;;
    esac
    ;;
  "for-each-ref --format=%(upstream:short) refs/heads/main")
    if [ "$NEKO_TEST_V1_PREFLIGHT_SCENARIO" != "no-upstream" ]; then printf 'origin/main\n'; fi
    ;;
  "status -sb")
    if [ "$NEKO_TEST_V1_PREFLIGHT_SCENARIO" = "behind" ]; then
      printf '## main...origin/main [behind 1]\n'
    else
      printf '## main...origin/main\n'
    fi
    ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake scenario git: %v", err)
	}
}
