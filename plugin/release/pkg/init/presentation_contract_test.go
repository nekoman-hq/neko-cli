package init

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestInitializeV2PresentationSeparatesDefaultAndDescribe(t *testing.T) {
	result := initializeV2Result{Unit: setupPresentationUnit("api")}
	response := mapInitializeV2Response(result, nil, time.Time{})

	plain := renderSetupResponse(t, response, false)
	described := renderSetupResponse(t, response, true)
	for _, want := range []string{
		"Release Initialization", "Initialized unit", "api", "Version", "1.2.3",
		"Executor", "goreleaser", "Delivery", "github-actions",
		"Configuration", ".neko/release.config.json", "State", ".neko/release.state.json", "Next action",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("init default omitted %q:\n%s", want, plain)
		}
	}
	for _, hidden := range []string{"Resolved Configuration", "Artifact Write Plan", "Validation Facts", "Limitations"} {
		if strings.Contains(plain, hidden) || !strings.Contains(described, hidden) {
			t.Fatalf("init describe visibility for %q is incorrect:\nplain:\n%s\ndescribed:\n%s", hidden, plain, described)
		}
	}
	for _, want := range []string{"Display name", "Tag prefix", "Working directory", "Declared paths", "Force behavior"} {
		if !strings.Contains(described, want) {
			t.Fatalf("init describe omitted %q:\n%s", want, described)
		}
	}
	assertSetupJSONPresentationInvariant(t, response)
}

func TestAddV2UnitPresentationSeparatesDefaultAndDescribe(t *testing.T) {
	result := addV2UnitResult{Unit: setupPresentationUnit("web")}
	response := mapAddV2UnitResponse(result, nil, time.Time{})

	plain := renderSetupResponse(t, response, false)
	described := renderSetupResponse(t, response, true)
	for _, want := range []string{
		"Release Unit Added", "Added unit", "web", "Version", "1.2.3",
		"Executor", "goreleaser", "Delivery", "github-actions",
		"Configuration", ".neko/release.config.json", "State", ".neko/release.state.json", "Next action",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("unit-add default omitted %q:\n%s", want, plain)
		}
	}
	for _, hidden := range []string{"Resolved Unit", "Existing Unit Comparison", "Artifact Write Plan", "Validation Facts", "Limitations"} {
		if strings.Contains(plain, hidden) || !strings.Contains(described, hidden) {
			t.Fatalf("unit-add describe visibility for %q is incorrect:\nplain:\n%s\ndescribed:\n%s", hidden, plain, described)
		}
	}
	assertSetupJSONPresentationInvariant(t, response)
}

func TestInitializationConflictsHaveActionablePresentation(t *testing.T) {
	tests := []struct {
		name        string
		failure     *commandFailure
		want        []string
		notExpected string
	}{
		{
			name: "init existing",
			failure: &commandFailure{
				code: "CONFIG_EXISTS", message: ".neko/release.config.json already exists",
				origin: commandFailureFromPresencePolicy,
			},
			want: []string{"Conflict", "V2 configuration", "Force applicable", "Yes", "--force"},
		},
		{
			name: "duplicate unit",
			failure: &commandFailure{
				code: "DUPLICATE_UNIT", message: `release unit "api" already exists in state`,
			},
			want:        []string{"Conflict", "Duplicate unit", "Force applicable", "No", "Choose a new unit id"},
			notExpected: "--force to overwrite",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := mapCommandFailure(test.failure, time.Time{})
			output := renderSetupResponse(t, response, false)
			for _, want := range test.want {
				if !strings.Contains(output, want) {
					t.Fatalf("conflict output omitted %q:\n%s", want, output)
				}
			}
			if test.notExpected != "" && strings.Contains(output, test.notExpected) {
				t.Fatalf("conflict output contains %q:\n%s", test.notExpected, output)
			}
		})
	}
}

func TestInitAndUnitAddVerbosePhasesAreChronologicalAndSafe(t *testing.T) {
	initRoot := newSetupPresentationRepository(t)
	initOutput := captureSetupLogs(t, func() {
		response, err := HandleInitAt(initRoot, plugin.Request{Flags: setupPresentationFlags("api")})
		if err != nil || response.Status != "success" {
			t.Fatalf("init response=%#v err=%v", response, err)
		}
	})
	assertSetupLogOrder(t, initOutput, []string{
		"Validating initialization command inputs",
		"Inspecting repository initialization state",
		"Resolving V2 release unit configuration",
		"Validating generated V2 configuration and state",
		"Preparing V2 configuration and state artifacts",
		"Writing V2 configuration and state",
		"V2 configuration and state write completed",
		"Initialization completed successfully",
	})

	unitRoot := newSetupPresentationRepository(t)
	repository := newV2Repository(unitRoot.Path())
	base, err := constructV2Unit(parseV2UnitRequest(setupPresentationFlags("api")))
	if err != nil {
		t.Fatalf("construct base unit: %v", err)
	}
	if err := repository.PersistPair(newV2ReleasePair(base)); err != nil {
		t.Fatalf("persist base pair: %v", err)
	}
	unitOutput := captureSetupLogs(t, func() {
		response, err := HandleUnitAddAt(unitRoot, plugin.Request{Flags: setupPresentationFlags("web")})
		if err != nil || response.Status != "success" {
			t.Fatalf("unit-add response=%#v err=%v", response, err)
		}
	})
	assertSetupLogOrder(t, unitOutput, []string{
		"Validating release unit inputs",
		"Inspecting existing V2 configuration and state",
		"Resolving release unit defaults",
		"Reading existing V2 configuration and state",
		"Checking duplicate unit identity",
		"Validating updated V2 configuration and state",
		"Writing updated V2 configuration and state",
		"Release unit append completed",
	})
	for _, output := range []string{initOutput, unitOutput} {
		for _, forbidden := range []string{initRoot.Path(), unitRoot.Path(), "GITHUB_TOKEN", "Authorization", "\x1b["} {
			if strings.Contains(output, forbidden) {
				t.Fatalf("verbose output contains %q:\n%s", forbidden, output)
			}
		}
	}
}

func setupPresentationUnit(id string) v2InitConfig {
	return v2InitConfig{
		UnitID: id, DisplayName: strings.ToUpper(id), Version: "1.2.3",
		Executor: releaseconfig.ExecutorGoReleaser, Delivery: releaseconfig.DeliveryGitHubActions,
		Workflow: ".github/workflows/release-" + id + ".yml", TagPrefix: id + "/v",
		WorkingDirectory: ".", Paths: []string{"services/" + id + "/**"}, Kind: defaultKind,
	}
}

func setupPresentationFlags(id string) map[string]any {
	return map[string]any{
		"unit": id, "display-name": strings.ToUpper(id), "version": "1.2.3",
		"executor": "goreleaser", "delivery": "github-actions",
		"workflow":   ".github/workflows/release-" + id + ".yml",
		"tag-prefix": id + "/v", "working-directory": ".", "paths": "services/" + id + "/**",
	}
}

func newSetupPresentationRepository(t *testing.T) workspace.RepositoryRoot {
	t.Helper()
	rootPath := t.TempDir()
	for _, id := range []string{"api", "web"} {
		target := filepath.Join(rootPath, ".github", "workflows", "release-"+id+".yml")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("create workflow directory: %v", err)
		}
		if err := os.WriteFile(target, []byte("name: release\n"), 0o644); err != nil {
			t.Fatalf("write workflow: %v", err)
		}
	}
	root, err := workspace.ValidateRepositoryRoot(rootPath)
	if err != nil {
		t.Fatalf("validate repository root: %v", err)
	}
	return root
}

func renderSetupResponse(t *testing.T, response *plugin.Response, describe bool) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: describe, WidthProvider: setupPresentationWidth(120),
	}, &output); err != nil {
		t.Fatalf("render setup response: %v", err)
	}
	return output.String()
}

func assertSetupJSONPresentationInvariant(t *testing.T, response *plugin.Response) {
	t.Helper()
	var plain bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{Format: renderer.FormatJSON}, &plain); err != nil {
		t.Fatalf("render plain JSON: %v", err)
	}
	var described bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
		Format: renderer.FormatJSON, Describe: true,
	}, &described); err != nil {
		t.Fatalf("render described JSON: %v", err)
	}
	if !reflect.DeepEqual(plain.Bytes(), described.Bytes()) {
		t.Fatalf("describe changed JSON:\nplain=%s\ndescribed=%s", plain.String(), described.String())
	}
	for _, forbidden := range []string{"human_table", "human_properties", "describe_only"} {
		if strings.Contains(plain.String(), forbidden) {
			t.Fatalf("public JSON contains %q:\n%s", forbidden, plain.String())
		}
	}
}

func captureSetupLogs(t *testing.T, run func()) string {
	t.Helper()
	previousVerbose := log.Verbose
	log.Verbose = true
	t.Cleanup(func() { log.Verbose = previousVerbose })
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	previousStderr := os.Stderr
	os.Stderr = writer
	run()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	os.Stderr = previousStderr
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}
	return ansi.Strip(string(content))
}

func assertSetupLogOrder(t *testing.T, output string, phases []string) {
	t.Helper()
	previous := -1
	for _, phase := range phases {
		index := strings.Index(output, phase)
		if index < 0 {
			t.Fatalf("verbose output omitted %q:\n%s", phase, output)
		}
		if index <= previous {
			t.Fatalf("verbose phase %q is out of order:\n%s", phase, output)
		}
		previous = index
	}
}

type setupPresentationWidth int

func (width setupPresentationWidth) Width(io.Writer) (int, bool) {
	return int(width), true
}
