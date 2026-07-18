package release

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const (
	pluginReleaseUnitID       = "plugin-release"
	pluginUIUnitID            = "plugin-ui"
	pluginReleaseManifestPath = "plugin/release/manifest.json"
	pluginUIManifestPath      = "plugin/ui/manifest.json"
)

func TestJReleaserMaterializerPlansOnlyJReleaserYML(t *testing.T) {
	root := newV2MaterializationRepository(t, "jreleaser")
	ctx := mustBuildTransactionContext(t, root, Minor)
	plan, err := JReleaserMaterializer{}.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := (JReleaserMaterializer{}).Validate(plan); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("expected one change, got %#v", plan.Changes)
	}
	change := plan.Changes[0]
	if change.RepositoryRelativePath != "jreleaser.yml" {
		t.Fatalf("expected jreleaser.yml, got %#v", change)
	}
	if !change.RequiredForReleaseCommit {
		t.Fatalf("expected jreleaser.yml to be required for release commit")
	}
	if !strings.Contains(string(change.AfterContent), "version: 0.3.0") {
		t.Fatalf("expected next version in materialized content:\n%s", string(change.AfterContent))
	}
	if mustReadString(t, filepath.Join(root, "jreleaser.yml")) != string(change.BeforeContent) {
		t.Fatal("Plan must not write jreleaser.yml")
	}
}

func TestGoReleaserMaterializerIsNoop(t *testing.T) {
	root := newV2MaterializationRepository(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	plan, err := GoReleaserMaterializer{}.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := (GoReleaserMaterializer{}).Validate(plan); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(plan.Changes) != 0 {
		t.Fatalf("expected no goreleaser materialization changes, got %#v", plan.Changes)
	}
}

func TestGoReleaserMaterializerSkipsCLIUnit(t *testing.T) {
	root := newV2MaterializationRepository(t, "goreleaser")
	cfg := `{"schemaVersion":2,"units":[{"id":"cli","paths":["**"],"workingDirectory":".","tagPrefix":"v","executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-api.yml"}}]}`
	state := `{"schemaVersion":2,"units":{"cli":{"version":"2.2.4"}}}`
	if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(cfg), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(releaseconfig.V2StatePath(root), []byte(state), 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	ctx := mustBuildTransactionContext(t, root, Patch)
	plan, err := GoReleaserMaterializer{}.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := (GoReleaserMaterializer{}).Validate(plan); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(plan.Changes) != 0 {
		t.Fatalf("cli unit must not receive plugin materialization changes, got %#v", plan.Changes)
	}
}

func TestPluginReleaseMaterializerPlansManifestFiles(t *testing.T) {
	root := newPluginReleaseMaterializationRepository(t)
	ctx := mustBuildTransactionContext(t, root, Patch)
	manifestBefore := mustReadString(t, filepath.Join(root, pluginReleaseManifestPath))

	plan, err := GoReleaserMaterializer{}.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := (GoReleaserMaterializer{}).Validate(plan); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("expected one plugin-release materialization change, got %#v", plan.Changes)
	}
	if !sameStringSet(materializationChangePaths(plan), []string{pluginReleaseManifestPath}) {
		t.Fatalf("unexpected materialized files: %#v", materializationChangePaths(plan))
	}
	for _, change := range plan.Changes {
		if !change.RequiredForReleaseCommit {
			t.Fatalf("expected %s to be required for release commit", change.RepositoryRelativePath)
		}
	}

	manifestChange := materializationChangeByPath(t, plan, pluginReleaseManifestPath)
	var beforeManifest map[string]json.RawMessage
	var afterManifest map[string]json.RawMessage
	if err := json.Unmarshal([]byte(manifestBefore), &beforeManifest); err != nil {
		t.Fatalf("decode before manifest: %v", err)
	}
	if err := json.Unmarshal(manifestChange.AfterContent, &afterManifest); err != nil {
		t.Fatalf("decode after manifest: %v", err)
	}
	var manifestVersion string
	if err := json.Unmarshal(afterManifest["version"], &manifestVersion); err != nil {
		t.Fatalf("decode manifest version: %v", err)
	}
	if manifestVersion != "3.0.1" {
		t.Fatalf("expected manifest version 3.0.1, got %s", manifestVersion)
	}
	for key, beforeValue := range beforeManifest {
		if key == "version" {
			continue
		}
		if !bytes.Equal(beforeValue, afterManifest[key]) {
			t.Fatalf("manifest field %s changed unexpectedly\nbefore: %s\nafter:  %s", key, beforeValue, afterManifest[key])
		}
	}

	if got := mustReadString(t, filepath.Join(root, pluginReleaseManifestPath)); got != manifestBefore {
		t.Fatal("Plan must not write plugin/release/manifest.json")
	}
}

func TestPluginUIReleaseMaterializerPlansOnlyUIManifest(t *testing.T) {
	root := newPluginUIReleaseMaterializationRepository(t)
	ctx := mustBuildTransactionContext(t, root, Patch)
	releaseManifestBefore := mustReadString(t, filepath.Join(root, pluginReleaseManifestPath))
	uiManifestBefore := mustReadString(t, filepath.Join(root, pluginUIManifestPath))

	plan, err := GoReleaserMaterializer{}.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := (GoReleaserMaterializer{}).Validate(plan); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !sameStringSet(materializationChangePaths(plan), []string{pluginUIManifestPath}) {
		t.Fatalf("unexpected materialized files: %#v", materializationChangePaths(plan))
	}
	uiChange := materializationChangeByPath(t, plan, pluginUIManifestPath)
	var afterManifest map[string]json.RawMessage
	if err := json.Unmarshal(uiChange.AfterContent, &afterManifest); err != nil {
		t.Fatalf("decode after manifest: %v", err)
	}
	var manifestVersion string
	if err := json.Unmarshal(afterManifest["version"], &manifestVersion); err != nil {
		t.Fatalf("decode manifest version: %v", err)
	}
	if manifestVersion != "1.0.1" {
		t.Fatalf("expected ui manifest version 1.0.1, got %s", manifestVersion)
	}
	if got := mustReadString(t, filepath.Join(root, pluginReleaseManifestPath)); got != releaseManifestBefore {
		t.Fatal("plugin-ui plan must not write plugin/release/manifest.json")
	}
	if got := mustReadString(t, filepath.Join(root, pluginUIManifestPath)); got != uiManifestBefore {
		t.Fatal("Plan must not write plugin/ui/manifest.json")
	}
}

func TestGoReleaserMaterializerUsesConfiguredManifestForArbitraryPluginUnit(t *testing.T) {
	root := newPluginReleaseMaterializationRepository(t)
	manifestPath := "extensions/audit/manifest.json"
	manifest := "{\n  \"name\": \"audit\",\n  \"version\": \"2.4.6\",\n  \"description\": \"Audit plugin\"\n}\n"
	if err := os.MkdirAll(filepath.Join(root, "extensions", "audit"), 0755); err != nil {
		t.Fatalf("mkdir arbitrary plugin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, manifestPath), []byte(manifest), 0644); err != nil {
		t.Fatalf("write arbitrary plugin manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".github", "workflows", "release-plugin-audit.yml"), []byte("name: release audit\n"), 0644); err != nil {
		t.Fatalf("write arbitrary plugin workflow: %v", err)
	}
	config := `{"schemaVersion":2,"units":[{"id":"plugin-audit","paths":["extensions/audit/**"],"workingDirectory":".","tagPrefix":"plugin-audit/v","kind":"plugin","plugin":{"name":"audit","manifest":"extensions/audit/manifest.json","assetPrefix":"plugin-audit","binaryName":"plugin-audit"},"executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-plugin-audit.yml"}}]}`
	state := `{"schemaVersion":2,"units":{"plugin-audit":{"version":"2.4.6"}}}`
	if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(config), 0644); err != nil {
		t.Fatalf("write arbitrary plugin config: %v", err)
	}
	if err := os.WriteFile(releaseconfig.V2StatePath(root), []byte(state), 0644); err != nil {
		t.Fatalf("write arbitrary plugin state: %v", err)
	}

	ctx := mustBuildTransactionContext(t, root, Patch)
	plan, err := GoReleaserMaterializer{}.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := (GoReleaserMaterializer{}).Validate(plan); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !sameStringSet(materializationChangePaths(plan), []string{manifestPath}) {
		t.Fatalf("unexpected materialized files: %#v", materializationChangePaths(plan))
	}
	change := materializationChangeByPath(t, plan, manifestPath)
	if got := string(change.AfterContent); got != strings.Replace(manifest, `"version": "2.4.6"`, `"version": "2.4.7"`, 1) {
		t.Fatalf("arbitrary plugin manifest bytes changed unexpectedly:\n%s", got)
	}
	if got := mustReadString(t, filepath.Join(root, manifestPath)); got != manifest {
		t.Fatal("Plan must not write the configured arbitrary plugin manifest")
	}
}

func TestPluginReleaseMaterializerFailsClearlyForRequiredFiles(t *testing.T) {
	tests := []struct { //nolint:govet // Test table order follows failure scenario readability.
		name       string
		path       string
		content    string
		removeFile bool
		want       string
	}{
		{
			name:    "malformed manifest",
			path:    pluginReleaseManifestPath,
			content: `{"version":`,
			want:    "parse plugin/release/manifest.json",
		},
		{
			name:       "missing manifest",
			path:       pluginReleaseManifestPath,
			removeFile: true,
			want:       "required plugin manifest file plugin/release/manifest.json not found for unit plugin-release",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newPluginReleaseMaterializationRepository(t)
			ctx := mustBuildTransactionContext(t, root, Patch)
			target := filepath.Join(root, tt.path)
			if tt.removeFile {
				if err := os.Remove(target); err != nil {
					t.Fatalf("remove %s: %v", tt.path, err)
				}
			} else if err := os.WriteFile(target, []byte(tt.content), 0644); err != nil {
				t.Fatalf("write %s: %v", tt.path, err)
			}
			_, err := GoReleaserMaterializer{}.Plan(ctx)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestReleaseItMaterializerAdvertisesBlockOnly(t *testing.T) {
	root := newV2MaterializationRepository(t, "release-it")
	ctx := mustBuildTransactionContext(t, root, Patch)
	plan, err := ReleaseItMaterializer{}.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Changes) != 0 {
		t.Fatalf("release-it must not materialize files for V2 real execution, got %#v", plan.Changes)
	}
	if plan.BlockedReason == "" {
		t.Fatal("expected release-it blocked reason")
	}
}

func TestMaterializationPlanRejectsOutsideRepository(t *testing.T) {
	root := newV2MaterializationRepository(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	plan := newMaterializationPlan(ctx)
	plan.Changes = []MaterializedFileChange{{
		AbsolutePath:           outside,
		RepositoryRelativePath: "../outside.txt",
		AfterContent:           []byte("x"),
		Reason:                 "test outside repository rejection",
	}}
	if err := ValidateMaterializationPlan(&plan); err == nil {
		t.Fatal("expected outside repository error")
	}
}

func TestMaterializationPlanRejectsDuplicateTargets(t *testing.T) {
	root := newV2MaterializationRepository(t, "jreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	path := filepath.Join(root, "jreleaser.yml")
	change, err := newMaterializedFileChange(ctx, path, []byte("a"), []byte("b"), 0644, true, "test duplicate", true)
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	plan := newMaterializationPlan(ctx)
	plan.Changes = []MaterializedFileChange{change, change}
	if err := ValidateMaterializationPlan(&plan); err == nil {
		t.Fatal("expected duplicate target error")
	}
}

func TestJReleaserMaterializerRejectsSymlinkedMaterializedFile(t *testing.T) {
	root := newV2MaterializationRepository(t, "jreleaser")
	realConfig := filepath.Join(root, "real-jreleaser.yml")
	if err := os.Rename(filepath.Join(root, "jreleaser.yml"), realConfig); err != nil {
		t.Fatalf("move jreleaser config: %v", err)
	}
	if err := os.Symlink(realConfig, filepath.Join(root, "jreleaser.yml")); err != nil {
		t.Fatalf("symlink jreleaser config: %v", err)
	}
	ctx := mustBuildTransactionContext(t, root, Patch)

	_, err := JReleaserMaterializer{}.Plan(ctx)
	if err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("expected symlink materialization error, got %v", err)
	}
}

func newV2MaterializationRepository(t *testing.T, executor string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".neko"), 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".goreleaser.yml"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write goreleaser config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".release-it.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write release-it config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "jreleaser.yml"), []byte("project:\n  name: api\n  version: 0.2.0\nrelease:\n  github:\n    owner: nekoman-hq\n"), 0644); err != nil {
		t.Fatalf("write jreleaser config: %v", err)
	}
	mustWriteReleaseTestFile(t, root, ".github/workflows/release-api.yml", "name: release api\n")
	cfg := `{"schemaVersion":2,"units":[{"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"` + executor + `","delivery":"github-actions","workflow":".github/workflows/release-api.yml"}}]}`
	state := `{"schemaVersion":2,"units":{"api":{"version":"0.2.0"}}}`
	if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(cfg), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(releaseconfig.V2StatePath(root), []byte(state), 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	return root
}

func newPluginReleaseMaterializationRepository(t *testing.T) string {
	t.Helper()
	return newPluginManifestMaterializationRepository(t, pluginReleaseUnitID)
}

func newPluginUIReleaseMaterializationRepository(t *testing.T) string {
	t.Helper()
	return newPluginManifestMaterializationRepository(t, pluginUIUnitID)
}

func newPluginManifestMaterializationRepository(t *testing.T, unitID string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".neko"), 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plugin", "release"), 0755); err != nil {
		t.Fatalf("mkdir plugin/release: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plugin", "ui"), 0755); err != nil {
		t.Fatalf("mkdir plugin/ui: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".goreleaser.yml"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write goreleaser config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".github", "workflows", "release-plugin-release.yml"), []byte("name: release plugin\n"), 0644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".github", "workflows", "release-plugin-ui.yml"), []byte("name: release ui\n"), 0644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	releaseManifest := `{
  "name": "release",
  "version": "3.0.0",
  "description": "Release management plugin",
  "author": "nekoman-hq",
  "commands": [
    {
      "name": "patch",
      "description": "Create a patch release"
    }
  ],
  "renderer_types": [
    "table",
    "json",
    "text"
  ]
}
`
	if err := os.WriteFile(filepath.Join(root, pluginReleaseManifestPath), []byte(releaseManifest), 0644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}
	uiManifest := `{
  "name": "ui",
  "version": "1.0.0",
  "description": "UI plugin",
  "author": "nekoman-hq",
  "commands": [
    {
      "name": "hello",
      "description": "Say hello"
    }
  ],
  "renderer_types": [
    "table",
    "json",
    "text"
  ]
}
`
	if err := os.WriteFile(filepath.Join(root, pluginUIManifestPath), []byte(uiManifest), 0644); err != nil {
		t.Fatalf("write ui manifest: %v", err)
	}
	pluginName := "release"
	manifestPath := pluginReleaseManifestPath
	version := "3.0.0"
	if unitID == pluginUIUnitID {
		pluginName = "ui"
		manifestPath = pluginUIManifestPath
		version = "1.0.0"
	}
	cfg := `{"schemaVersion":2,"units":[{"id":"` + unitID + `","paths":["plugin/**"],"workingDirectory":".","tagPrefix":"` + unitID + `/v","kind":"plugin","plugin":{"name":"` + pluginName + `","manifest":"` + manifestPath + `","assetPrefix":"` + unitID + `","binaryName":"` + unitID + `"},"executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-` + unitID + `.yml"}}]}`
	state := `{"schemaVersion":2,"units":{"` + unitID + `":{"version":"` + version + `"}}}`
	if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(cfg), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(releaseconfig.V2StatePath(root), []byte(state), 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	return root
}

func materializationChangePaths(plan *MaterializationPlan) []string {
	paths := make([]string, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		paths = append(paths, filepath.ToSlash(change.RepositoryRelativePath))
	}
	return paths
}

func materializationChangeByPath(t *testing.T, plan *MaterializationPlan, path string) MaterializedFileChange {
	t.Helper()
	for _, change := range plan.Changes {
		if filepath.ToSlash(change.RepositoryRelativePath) == path {
			return change
		}
	}
	t.Fatalf("materialization change %s not found in %#v", path, materializationChangePaths(plan))
	return MaterializedFileChange{}
}
