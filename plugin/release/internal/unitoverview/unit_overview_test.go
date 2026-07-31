package unitoverview

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestUnitOverviewListsCanonicalUnitsDeterministically(t *testing.T) {
	root := newUnitOverviewRepository(t)
	config := releaseconfig.V2ReleaseConfig{SchemaVersion: 2, Units: []releaseconfig.V2Unit{
		unitOverviewConfigUnit("worker", "Worker", "worker/v", ".github/workflows/release-shared.yml"),
		unitOverviewConfigUnit("cli", "Neko CLI", "v", ".github/workflows/release-cli.yml"),
		unitOverviewConfigUnit("api", "", "api/v", ".github/workflows/release-shared.yml"),
	}}
	state := releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{
		"worker": {Version: "2.0.0"},
		"cli":    {Version: "3.0.5"},
		"api":    {Version: "1.2.3"},
	}}
	writeUnitOverviewPair(t, root.Path(), config, state)

	response := runUnitOverview(t, root)
	result := unitOverviewResponseResult(t, response)
	code, present := response.ExplicitExitCode()
	if !present || code != 0 || result.status != unitOverviewValid {
		t.Fatalf("status=%q exit=%d", result.status, response.ExitCode)
	}
	if got := unitOverviewRowIDs(result.units); !reflect.DeepEqual(got, []string{"api", "cli", "worker"}) {
		t.Fatalf("unit order = %#v", got)
	}
	if result.summary.Total != 3 || result.summary.Aligned != 3 || result.summary.WorkflowPaths != 2 || !result.summary.SourceUsable {
		t.Fatalf("summary = %#v", result.summary)
	}
	if !reflect.DeepEqual(result.workflowPaths, []string{
		".github/workflows/release-cli.yml",
		".github/workflows/release-shared.yml",
	}) {
		t.Fatalf("workflow paths = %#v", result.workflowPaths)
	}
	cli := result.units[1]
	for key, want := range map[string]any{
		"display_name":       "Neko CLI",
		"version":            "3.0.5",
		"configured_version": "3.0.5",
		"tag_prefix":         "v",
		"tag_shape":          "v<version>",
		"configured_tag":     "v3.0.5",
		"alignment":          unitOverviewAligned,
	} {
		if got := cli[key]; got != want {
			t.Fatalf("cli[%s] = %#v, want %#v", key, got, want)
		}
	}
	if _, present := result.units[0]["display_name"]; present {
		t.Fatal("empty optional display_name must be absent")
	}
}

func TestUnitOverviewKeepsIncompleteUnitsVisible(t *testing.T) {
	root := newUnitOverviewRepository(t)
	config := releaseconfig.V2ReleaseConfig{SchemaVersion: 2, Units: []releaseconfig.V2Unit{
		unitOverviewConfigUnit("api", "API", "api/v", ".github/workflows/release-api.yml"),
	}}
	state := releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{
		"worker": {Version: "2.4.0"},
	}}
	writeUnitOverviewPair(t, root.Path(), config, state)

	response := runUnitOverview(t, root)
	result := unitOverviewResponseResult(t, response)
	code, present := response.ExplicitExitCode()
	if !present || code != 1 || result.status != unitOverviewHasIssues || result.summary.Incomplete != 2 {
		t.Fatalf("status=%q summary=%#v exit=%d", result.status, result.summary, response.ExitCode)
	}
	if got := unitOverviewRowIDs(result.units); !reflect.DeepEqual(got, []string{"api", "worker"}) {
		t.Fatalf("unit ids = %#v", got)
	}
	assertUnitOverviewRow(t, result.units[0], unitOverviewConfigOnly, "UNIT_STATE_MISSING")
	assertUnitOverviewRow(t, result.units[1], unitOverviewStateOnly, "UNIT_CONFIG_MISSING")
	if result.units[1]["version"] != "2.4.0" || result.units[1]["configured_version"] != "2.4.0" {
		t.Fatalf("state-only version facts = %#v", result.units[1])
	}
}

func TestUnitOverviewClassifiesInvalidUnitFacts(t *testing.T) {
	tests := []struct {
		name       string
		mutateUnit func(*releaseconfig.V2Unit)
		version    string
		code       string
	}{
		{name: "version", version: "future", code: "UNIT_VERSION_INVALID"},
		{name: "tag prefix", version: "1.2.3", mutateUnit: func(unit *releaseconfig.V2Unit) { unit.TagPrefix = "../unsafe" }, code: "UNIT_TAG_PREFIX_INVALID"},
		{name: "executor", version: "1.2.3", mutateUnit: func(unit *releaseconfig.V2Unit) { unit.Executor.Type = "unknown" }, code: "UNIT_EXECUTOR_INVALID"},
		{name: "delivery", version: "1.2.3", mutateUnit: func(unit *releaseconfig.V2Unit) { unit.Executor.Delivery = releaseconfig.DeliveryLocal }, code: "UNIT_DELIVERY_INVALID"},
		{name: "workflow path", version: "1.2.3", mutateUnit: func(unit *releaseconfig.V2Unit) { unit.Executor.Workflow = "workflows/release.yml" }, code: "UNIT_WORKFLOW_PATH_INVALID"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newUnitOverviewRepository(t)
			unit := unitOverviewConfigUnit("api", "API", "api/v", ".github/workflows/release-api.yml")
			if test.mutateUnit != nil {
				test.mutateUnit(&unit)
			}
			writeUnitOverviewPair(t, root.Path(),
				releaseconfig.V2ReleaseConfig{SchemaVersion: 2, Units: []releaseconfig.V2Unit{unit}},
				releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{"api": {Version: test.version}}},
			)

			response := runUnitOverview(t, root)
			result := unitOverviewResponseResult(t, response)
			if response.ExitCode != 1 || result.status != unitOverviewHasIssues || result.summary.Invalid != 1 {
				t.Fatalf("status=%q summary=%#v exit=%d", result.status, result.summary, response.ExitCode)
			}
			assertUnitOverviewRow(t, result.units[0], unitOverviewInvalid, test.code)
			if test.code == "UNIT_VERSION_INVALID" {
				if _, present := result.units[0]["version"]; present {
					t.Fatalf("invalid canonical version leaked: %#v", result.units[0])
				}
				if result.units[0]["configured_version"] != test.version {
					t.Fatalf("configured version not preserved: %#v", result.units[0])
				}
			}
		})
	}
}

func TestUnitOverviewValidatesTagPolicyIndependentlyOfOtherUnitErrors(t *testing.T) {
	root := newUnitOverviewRepository(t)
	unit := unitOverviewConfigUnit("API", "API", "../unsafe", ".github/workflows/release-api.yml")
	writeUnitOverviewPair(t, root.Path(),
		releaseconfig.V2ReleaseConfig{SchemaVersion: 2, Units: []releaseconfig.V2Unit{unit}},
		releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{"API": {Version: "1.2.3"}}},
	)

	result := unitOverviewResponseResult(t, runUnitOverview(t, root))
	if len(result.units) != 1 {
		t.Fatalf("units = %#v", result.units)
	}
	assertUnitOverviewRow(t, result.units[0], unitOverviewInvalid, "UNIT_TAG_PREFIX_INVALID")
	if _, present := result.units[0]["tag_shape"]; present {
		t.Fatalf("unsafe tag shape leaked: %#v", result.units[0])
	}
	if _, present := result.units[0]["configured_tag"]; present {
		t.Fatalf("unsafe configured tag leaked: %#v", result.units[0])
	}
}

func TestUnitOverviewClassifiesOverlappingTagPrefixesForEveryUnit(t *testing.T) {
	root := newUnitOverviewRepository(t)
	config := releaseconfig.V2ReleaseConfig{SchemaVersion: 2, Units: []releaseconfig.V2Unit{
		unitOverviewConfigUnit("api", "API", "service/v", ".github/workflows/release.yml"),
		unitOverviewConfigUnit("worker", "Worker", "service/v2", ".github/workflows/release.yml"),
	}}
	state := releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{
		"api": {Version: "1.2.3"}, "worker": {Version: "2.0.0"},
	}}
	writeUnitOverviewPair(t, root.Path(), config, state)

	result := unitOverviewResponseResult(t, runUnitOverview(t, root))
	if result.summary.Invalid != 2 || result.summary.WorkflowPaths != 1 {
		t.Fatalf("summary = %#v", result.summary)
	}
	for _, row := range result.units {
		assertUnitOverviewRow(t, row, unitOverviewInvalid, "UNIT_TAG_PREFIX_CONFLICT")
	}
}

func TestUnitOverviewReportsExpectedSourceStatesWithoutGoErrors(t *testing.T) {
	tests := []struct {
		setup    func(*testing.T, string)
		name     string
		code     string
		rowCount int
	}{
		{name: "empty source", code: "V2_SOURCE_MISSING", setup: func(*testing.T, string) {}},
		{name: "missing config", code: "V2_CONFIG_MISSING", rowCount: 1, setup: func(t *testing.T, root string) {
			writeUnitOverviewJSON(t, releaseconfig.V2StatePath(root), releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{"api": {Version: "1.2.3"}}})
		}},
		{name: "missing state", code: "V2_STATE_MISSING", rowCount: 1, setup: func(t *testing.T, root string) {
			writeUnitOverviewJSON(t, releaseconfig.V2ConfigPath(root), releaseconfig.V2ReleaseConfig{SchemaVersion: 2, Units: []releaseconfig.V2Unit{unitOverviewConfigUnit("api", "API", "api/v", ".github/workflows/release.yml")}})
		}},
		{name: "malformed config", code: "V2_CONFIG_INVALID", setup: func(t *testing.T, root string) {
			writeUnitOverviewBytes(t, releaseconfig.V2ConfigPath(root), []byte(`{"schemaVersion":2,`))
			writeUnitOverviewJSON(t, releaseconfig.V2StatePath(root), releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{}})
		}},
		{name: "malformed state", code: "V2_STATE_INVALID", setup: func(t *testing.T, root string) {
			writeUnitOverviewJSON(t, releaseconfig.V2ConfigPath(root), releaseconfig.V2ReleaseConfig{SchemaVersion: 2, Units: []releaseconfig.V2Unit{unitOverviewConfigUnit("api", "API", "api/v", ".github/workflows/release.yml")}})
			writeUnitOverviewBytes(t, releaseconfig.V2StatePath(root), []byte(`{"schemaVersion":2,`))
		}},
		{name: "unsupported schema", code: "V2_SCHEMA_UNSUPPORTED", setup: func(t *testing.T, root string) {
			writeUnitOverviewPair(t, root,
				releaseconfig.V2ReleaseConfig{SchemaVersion: 3, Units: []releaseconfig.V2Unit{unitOverviewConfigUnit("api", "API", "api/v", ".github/workflows/release.yml")}},
				releaseconfig.V2ReleaseState{SchemaVersion: 3, Units: map[string]releaseconfig.V2UnitState{"api": {Version: "1.2.3"}}},
			)
		}},
		{name: "v1 only", code: "V1_SOURCE_UNSUPPORTED", setup: func(t *testing.T, root string) {
			writeUnitOverviewBytes(t, filepath.Join(root, ".release.neko.json"), []byte(`{"version":"1.2.3"}`))
		}},
		{name: "mixed source", code: "MIXED_RELEASE_SOURCES", setup: func(t *testing.T, root string) {
			writeValidUnitOverviewPair(t, root)
			writeUnitOverviewBytes(t, filepath.Join(root, ".release.neko.json"), []byte(`{"version":"1.2.3"}`))
		}},
		{name: "recovery blocked", code: "V2_RECOVERY_BLOCKED", setup: func(t *testing.T, root string) {
			writeValidUnitOverviewPair(t, root)
			writeUnitOverviewBytes(t, releaseconfig.V2PairRecoveryPath(root), []byte("{}\n"))
		}},
		{name: "empty pair", code: "V2_SOURCE_EMPTY", setup: func(t *testing.T, root string) {
			writeUnitOverviewPair(t, root,
				releaseconfig.V2ReleaseConfig{SchemaVersion: 2, Units: []releaseconfig.V2Unit{}},
				releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{}},
			)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newUnitOverviewRepository(t)
			test.setup(t, root.Path())
			response := runUnitOverview(t, root)
			result := unitOverviewResponseResult(t, response)
			if response.ExitCode != 1 || result.status != unitOverviewSourceInvalid {
				t.Fatalf("status=%q exit=%d", result.status, response.ExitCode)
			}
			if result.sourceIssue == nil || result.sourceIssue.Code != test.code {
				t.Fatalf("source issue = %#v, want %s", result.sourceIssue, test.code)
			}
			if len(result.units) != test.rowCount {
				t.Fatalf("row count = %d, want %d", len(result.units), test.rowCount)
			}
		})
	}
}

func TestUnitOverviewJSONSchemaIsDeterministicAndPresentationFree(t *testing.T) {
	root := newUnitOverviewRepository(t)
	writeValidUnitOverviewPair(t, root.Path())
	response := runUnitOverview(t, root)

	first, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatalf("marshal first result: %v", err)
	}
	second, err := json.Marshal(runUnitOverview(t, root).Data)
	if err != nil {
		t.Fatalf("marshal second result: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("JSON is nondeterministic:\n%s\n%s", first, second)
	}
	for _, required := range []string{`"status"`, `"summary"`, `"units"`, `"workflow_paths"`, `"tag_shape"`, `"configured_tag"`, `"issues"`} {
		if !strings.Contains(string(first), required) {
			t.Fatalf("JSON omitted %s: %s", required, first)
		}
	}
	if !strings.Contains(string(first), `"issues":[]`) {
		t.Fatalf("valid unit issues must be a stable empty list: %s", first)
	}
	for _, forbidden := range []string{"human_table", "human_properties", "renderer_hint", "next_version", "planned_version", "tag_exists"} {
		if strings.Contains(string(first), forbidden) {
			t.Fatalf("JSON contains forbidden field %q: %s", forbidden, first)
		}
	}
}

func TestUnitOverviewJSONKeepsEmptyCollectionsAsArrays(t *testing.T) {
	root := newUnitOverviewRepository(t)
	response := runUnitOverview(t, root)
	encoded, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatalf("marshal empty-source response: %v", err)
	}
	for _, required := range []string{`"units":[]`, `"workflow_paths":[]`} {
		if !strings.Contains(string(encoded), required) {
			t.Fatalf("empty collection %s must remain an array: %s", required, encoded)
		}
	}
}

type unitOverviewResponseView struct {
	sourceIssue   *unitOverviewIssue
	status        unitOverviewStatus
	units         []map[string]any
	workflowPaths []string
	summary       unitOverviewSummary
}

func unitOverviewResponseResult(t *testing.T, response *plugin.Response) unitOverviewResponseView {
	t.Helper()
	view := unitOverviewResponseView{}
	var ok bool
	view.status, ok = response.Data["status"].(unitOverviewStatus)
	if !ok {
		t.Fatalf("status type = %T", response.Data["status"])
	}
	view.summary, ok = response.Data["summary"].(unitOverviewSummary)
	if !ok {
		t.Fatalf("summary type = %T", response.Data["summary"])
	}
	view.units, ok = response.Data["units"].([]map[string]any)
	if !ok {
		t.Fatalf("units type = %T", response.Data["units"])
	}
	view.workflowPaths, ok = response.Data["workflow_paths"].([]string)
	if !ok {
		t.Fatalf("workflow paths type = %T", response.Data["workflow_paths"])
	}
	if sourceIssue, present := response.Data["source_issue"]; present {
		issue, issueOK := sourceIssue.(unitOverviewIssue)
		if !issueOK {
			t.Fatalf("source issue type = %T", sourceIssue)
		}
		view.sourceIssue = &issue
	}
	return view
}

func assertUnitOverviewRow(t *testing.T, row map[string]any, alignment unitOverviewAlignment, code string) {
	t.Helper()
	if row["alignment"] != alignment {
		t.Fatalf("alignment = %#v, want %q", row["alignment"], alignment)
	}
	issues, ok := row["issues"].([]unitOverviewIssue)
	if !ok {
		t.Fatalf("issues type = %T", row["issues"])
	}
	for _, issue := range issues {
		if issue.Code == code {
			return
		}
	}
	t.Fatalf("issue %s missing from %#v", code, issues)
}

func unitOverviewRowIDs(rows []map[string]any) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		id, ok := row["id"].(string)
		if !ok {
			ids = append(ids, "")
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func unitOverviewConfigUnit(id, displayName, tagPrefix, workflow string) releaseconfig.V2Unit {
	return releaseconfig.V2Unit{
		ID: id, DisplayName: displayName, Paths: []string{id + "/**"},
		TagPrefix: tagPrefix, WorkingDirectory: ".",
		Executor: releaseconfig.V2Executor{
			Type: releaseconfig.ExecutorGoReleaser, Delivery: releaseconfig.DeliveryGitHubActions,
			Workflow: workflow,
		},
	}
}

func newUnitOverviewRepository(t *testing.T) workspace.RepositoryRoot {
	t.Helper()
	rootPath := t.TempDir()
	if err := os.Mkdir(filepath.Join(rootPath, ".git"), 0755); err != nil {
		t.Fatalf("create git marker: %v", err)
	}
	root, err := workspace.ResolveInspectionRepositoryRoot(rootPath)
	if err != nil {
		t.Fatalf("resolve inspection root: %v", err)
	}
	return root
}

func writeValidUnitOverviewPair(t *testing.T, root string) {
	t.Helper()
	writeUnitOverviewPair(t, root,
		releaseconfig.V2ReleaseConfig{SchemaVersion: 2, Units: []releaseconfig.V2Unit{unitOverviewConfigUnit("api", "API", "api/v", ".github/workflows/release-api.yml")}},
		releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{"api": {Version: "1.2.3"}}},
	)
}

func writeUnitOverviewPair(t *testing.T, root string, config releaseconfig.V2ReleaseConfig, state releaseconfig.V2ReleaseState) {
	t.Helper()
	writeUnitOverviewJSON(t, releaseconfig.V2ConfigPath(root), config)
	writeUnitOverviewJSON(t, releaseconfig.V2StatePath(root), state)
}

func writeUnitOverviewJSON(t *testing.T, path string, value any) {
	t.Helper()
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	writeUnitOverviewBytes(t, path, append(content, '\n'))
}

func writeUnitOverviewBytes(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create %s parent: %v", path, err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runUnitOverview(t *testing.T, root workspace.RepositoryRoot) *plugin.Response {
	t.Helper()
	response, err := HandleUnitsAt(root, plugin.Request{Command: unitOverviewCommandName})
	if err != nil {
		t.Fatalf("HandleUnitsAt: %v", err)
	}
	return response
}
