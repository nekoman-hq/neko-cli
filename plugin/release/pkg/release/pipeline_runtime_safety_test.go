package release

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPipelineRuntimeInspectionIsStrictlyReadOnly(t *testing.T) {
	fixture := newTaggedResumeRelease(t)
	identity := prepareAcceptedDispatchForResume(t, fixture)
	persistTagPushedExecution(t, fixture, identity)
	journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath)
	if err := journal.ConfirmPhase(ReleaseExecutionHandoffReady, ReleaseExecutionJournalUpdate{}, time.Now()); err != nil {
		t.Fatalf("confirm handoff: %v", err)
	}
	writeExecutionJournalFixture(t, fixture.executionPath, journal)

	before := capturePipelineRepositoryState(t, fixture)
	t.Setenv("GITHUB_TOKEN", "pipeline-inspection-must-not-read-this-secret")
	first := inspectPipelineRuntimeForTest(t, fixture.root)
	second := inspectPipelineRuntimeForTest(t, fixture.root)
	after := capturePipelineRepositoryState(t, fixture)
	if before != after {
		t.Fatalf("pipeline inspection mutated repository evidence:\nbefore=%#v\nafter=%#v", before, after)
	}
	if first.ExitCode != 0 || second.ExitCode != 0 {
		t.Fatalf("inspection failed: first=%#v second=%#v", first, second)
	}
	encoded, err := json.Marshal(first.Data)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	text := string(encoded)
	for _, forbidden := range []string{fixture.root, "pipeline-inspection-must-not-read-this-secret", "\x1b["} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("pipeline machine data contains forbidden value %q: %s", forbidden, text)
		}
	}
}

type pipelineRepositoryState struct {
	Head      string
	Status    string
	Refs      string
	Config    string
	State     string
	Execution string
	Dispatch  string
}

func capturePipelineRepositoryState(t *testing.T, fixture *resumeReleaseFixture) pipelineRepositoryState {
	t.Helper()
	return pipelineRepositoryState{
		Head:      strings.TrimSpace(gitOutput(t, fixture.root, "rev-parse", "HEAD")),
		Status:    gitOutput(t, fixture.root, "status", "--porcelain", "--untracked-files=all"),
		Refs:      gitOutput(t, fixture.root, "show-ref"),
		Config:    mustReadString(t, filepath.Join(fixture.root, ".neko", "release.config.json")),
		State:     mustReadString(t, filepath.Join(fixture.root, ".neko", "release.state.json")),
		Execution: mustReadString(t, fixture.executionPath),
		Dispatch:  mustReadString(t, fixture.dispatchPath),
	}
}

func TestPipelineRuntimeArchitectureHasNoMutationNetworkOrHandlerChaining(t *testing.T) {
	paths, err := filepath.Glob("pipeline_runtime*.go")
	if err != nil {
		t.Fatal(err)
	}
	policySource := ""
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		policySource += string(content)
		parsed, err := parser.ParseFile(token.NewFileSet(), path, content, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imported := range parsed.Imports {
			name, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"net", "net/http", "os/exec", "github.com/nekoman-hq/neko-cli/plugin/release/internal/githubdispatch"} {
				if name == forbidden || strings.HasPrefix(name, forbidden+"/") {
					t.Errorf("%s imports prohibited runtime capability %q", path, name)
				}
			}
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := pipelineRuntimeCallName(call)
			for _, forbidden := range []string{
				"WriteFile", "Mkdir", "MkdirAll", "Create", "Remove", "Rename",
				"Prepare", "Transition", "BeginPending", "ConfirmPhase", "RecordLastError",
				"Push", "Dispatch", "ResolveGitHubActionsDispatchToken",
				"HandleResume", "HandleEvidence", "HandleDoctor", "HandleValidate", "HandlePlan", "HandleUnits",
			} {
				if name == forbidden {
					t.Errorf("%s reaches prohibited mutation or handler capability %s", path, name)
				}
			}
			if name == "Slice" && len(call.Args) >= 2 {
				ast.Inspect(call.Args[1], func(node ast.Node) bool {
					selector, ok := node.(*ast.SelectorExpr)
					if ok && (selector.Sel.Name == "CreatedAt" || selector.Sel.Name == "UpdatedAt") {
						t.Errorf("%s uses journal timestamps for selection ordering", path)
					}
					return true
				})
			}
			return true
		})
	}
	for _, required := range []string{"AssessReleaseExecutionRecovery", "resolveResumeRecovery", "resolveResumeDispatch", "releaseExecutionStateOrder"} {
		if !strings.Contains(policySource, required) {
			t.Errorf("pipeline runtime does not reuse authoritative %s", required)
		}
	}
	for _, forbidden := range []string{"CanTransitionTo(", "ConfirmPhase(", "Transition(", "time.Since(", "sort.SliceStable("} {
		if strings.Contains(policySource, forbidden) {
			t.Errorf("pipeline runtime contains prohibited state-machine or recency behavior %q", forbidden)
		}
	}
}

func pipelineRuntimeCallName(call *ast.CallExpr) string {
	switch function := call.Fun.(type) {
	case *ast.Ident:
		return function.Name
	case *ast.SelectorExpr:
		return function.Sel.Name
	default:
		return ""
	}
}
