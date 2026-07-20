package release

import (
	"bytes"
	"go/ast"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/githubdispatch"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
)

const (
	githubDispatchImportPath  = "github.com/nekoman-hq/neko-cli/plugin/release/internal/githubdispatch"
	releaseWorkflowImportPath = "github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
)

func TestWorkflowBoundaryDependencyDirection(t *testing.T) {
	foundTransportFacade := false
	foundStaticFacts := false
	for _, parsed := range parseReleaseProductionFiles(t) {
		imports := releaseWorkflowArchitectureImports(t, parsed.file)
		foundStaticFacts = foundStaticFacts || imports[releaseWorkflowImportPath]
		if imports[githubDispatchImportPath] {
			foundTransportFacade = true
			if parsed.path != "github_actions_dispatch_client.go" {
				t.Errorf("%s imports the POST transport outside its root compatibility facade", parsed.path)
			}
		}
		if !strings.HasPrefix(parsed.path, "integration_doctor_") {
			continue
		}
		if imports[githubDispatchImportPath] {
			t.Errorf("Doctor production file %s imports the workflow-dispatch POST transport", parsed.path)
		}
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.Ident:
				for _, prohibited := range []string{
					"GitHubActionsDispatchClient", "NewGitHubActionsDispatchClient", "GitHubActionsDispatcher",
				} {
					if typed.Name == prohibited {
						t.Errorf("Doctor production file %s reaches POST capability %s", parsed.path, typed.Name)
					}
				}
			case *ast.SelectorExpr:
				identifier, ok := typed.X.(*ast.Ident)
				if ok && identifier.Name == "http" && typed.Sel.Name == "MethodPost" {
					t.Errorf("Doctor production file %s constructs an HTTP POST", parsed.path)
				}
			}
			return true
		})
	}
	if !foundTransportFacade {
		t.Fatal("root dispatch compatibility facade does not import the focused POST transport")
	}
	if !foundStaticFacts {
		t.Fatal("root release package does not consume the static workflow facts")
	}
}

func releaseWorkflowArchitectureImports(t *testing.T, file *ast.File) map[string]bool {
	t.Helper()
	imports := make(map[string]bool, len(file.Imports))
	for _, specification := range file.Imports {
		importPath, err := strconv.Unquote(specification.Path.Value)
		if err != nil {
			t.Fatalf("unquote import: %v", err)
		}
		imports[importPath] = true
	}
	return imports
}

func TestJournalAwareDispatcherOwnsDispatchTransitionTiming(t *testing.T) {
	method := findReleaseMethod(t, parseReleaseProductionFiles(t), "GitHubActionsDispatcher", "dispatch")
	operations := make([]string, 0, 3)
	ast.Inspect(method.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch {
		case selector.Sel.Name == "Transition" && callHasSecondIdentifier(call, "DispatchJournalRequestStarted"):
			operations = append(operations, "persist request-started")
		case selector.Sel.Name == "Dispatch" && selectorReceiverField(selector) == "client":
			operations = append(operations, "send POST")
		case selector.Sel.Name == "Transition" && callHasSecondSelector(call, "response", "State"):
			operations = append(operations, "persist terminal outcome")
		}
		return true
	})
	want := []string{"persist request-started", "send POST", "persist terminal outcome"}
	if !reflect.DeepEqual(operations, want) {
		t.Fatalf("dispatch transition order = %v, want %v", operations, want)
	}
}

func selectorReceiverField(selector *ast.SelectorExpr) string {
	receiver, ok := selector.X.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	return receiver.Sel.Name
}

func callHasSecondIdentifier(call *ast.CallExpr, name string) bool {
	if len(call.Args) < 2 {
		return false
	}
	identifier, ok := call.Args[1].(*ast.Ident)
	return ok && identifier.Name == name
}

func callHasSecondSelector(call *ast.CallExpr, receiverName, fieldName string) bool {
	if len(call.Args) < 2 {
		return false
	}
	selector, ok := call.Args[1].(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != fieldName {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	return ok && receiver.Name == receiverName
}

func TestDispatchJournalTransitionOwnershipRemainsRoot(t *testing.T) {
	files := parseReleaseProductionFiles(t)
	stateDeclarations := 0
	for _, parsed := range files {
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			declaration, ok := node.(*ast.TypeSpec)
			if ok && declaration.Name.Name == "DispatchJournalState" {
				stateDeclarations++
			}
			return true
		})
	}
	if stateDeclarations != 1 {
		t.Fatalf("DispatchJournalState declarations = %d, want 1", stateDeclarations)
	}
	_ = findReleaseMethod(t, files, "DispatchJournal", "Transition")
	_ = findReleaseMethod(t, files, "DispatchJournalStore", "Transition")
}

func TestRootWorkflowCompatibilityWrappersDelegateToStaticFacts(t *testing.T) {
	if GitHubActionsReleaseWorkflowContractVersion != releaseworkflow.GitHubActionsReleaseWorkflowContractVersion {
		t.Fatal("root workflow contract version is not the static fact")
	}
	wrapper, err := RenderCanonicalGitHubActionsReleaseWorkflow()
	if err != nil {
		t.Fatalf("root renderer: %v", err)
	}
	canonical, err := releaseworkflow.RenderCanonicalGitHubActionsReleaseWorkflow()
	if err != nil {
		t.Fatalf("static renderer: %v", err)
	}
	if !bytes.Equal(wrapper, canonical) {
		t.Fatal("root workflow renderer changed canonical bytes")
	}
	wrapperTarget, err := ResolveGitHubRepositoryTarget("origin", "git@github.com:owner/repository.git")
	if err != nil {
		t.Fatalf("root target resolver: %v", err)
	}
	canonicalTarget, err := releaseworkflow.ResolveGitHubRepositoryTarget("origin", "git@github.com:owner/repository.git")
	if err != nil {
		t.Fatalf("static target resolver: %v", err)
	}
	if !reflect.DeepEqual(wrapperTarget, canonicalTarget) {
		t.Fatalf("root target = %#v, canonical = %#v", wrapperTarget, canonicalTarget)
	}
}

func TestNeutralDispatchOutcomesMapOnlyToTerminalJournalStates(t *testing.T) {
	tests := map[githubdispatch.Outcome]DispatchJournalState{
		githubdispatch.OutcomeAccepted: DispatchJournalAccepted,
		githubdispatch.OutcomeRejected: DispatchJournalRejected,
		githubdispatch.OutcomeUnknown:  DispatchJournalUnknown,
	}
	for outcome, want := range tests {
		if got := dispatchJournalStateForOutcome(outcome); got != want {
			t.Errorf("outcome %q maps to %q, want %q", outcome, got, want)
		}
	}
	for _, prohibited := range []DispatchJournalState{DispatchJournalPrepared, DispatchJournalRequestStarted} {
		for outcome := range tests {
			if dispatchJournalStateForOutcome(outcome) == prohibited {
				t.Errorf("neutral outcome %q maps to non-terminal state %q", outcome, prohibited)
			}
		}
	}
}
