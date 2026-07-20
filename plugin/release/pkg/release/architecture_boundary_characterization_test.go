package release

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestReleaseExecutionJournalTransitionGraphContract(t *testing.T) {
	wantStates := []ReleaseExecutionJournalState{
		ReleaseExecutionPrepared,
		ReleaseExecutionPreflightValidated,
		ReleaseExecutionMaterializationApplied,
		ReleaseExecutionStateWritten,
		ReleaseExecutionReleaseFilesStaged,
		ReleaseExecutionCommitCreated,
		ReleaseExecutionTagCreated,
		ReleaseExecutionDispatchJournalPrepared,
		ReleaseExecutionCommitPushed,
		ReleaseExecutionTagPushed,
		ReleaseExecutionHandoffReady,
	}
	if !reflect.DeepEqual(releaseExecutionStateOrder, wantStates) {
		t.Fatalf("execution state order = %v, want %v", releaseExecutionStateOrder, wantStates)
	}

	wantPending := map[ReleaseExecutionJournalState]ReleaseExecutionPendingAction{
		ReleaseExecutionPrepared:                ReleaseExecutionPendingNone,
		ReleaseExecutionPreflightValidated:      ReleaseExecutionPendingNone,
		ReleaseExecutionMaterializationApplied:  ReleaseExecutionPendingApplyMaterialization,
		ReleaseExecutionStateWritten:            ReleaseExecutionPendingWriteState,
		ReleaseExecutionReleaseFilesStaged:      ReleaseExecutionPendingStageReleaseFiles,
		ReleaseExecutionCommitCreated:           ReleaseExecutionPendingCreateReleaseCommit,
		ReleaseExecutionTagCreated:              ReleaseExecutionPendingCreateUnitTag,
		ReleaseExecutionDispatchJournalPrepared: ReleaseExecutionPendingCreateDispatchJournal,
		ReleaseExecutionCommitPushed:            ReleaseExecutionPendingPushReleaseCommit,
		ReleaseExecutionTagPushed:               ReleaseExecutionPendingPushUnitTag,
		ReleaseExecutionHandoffReady:            ReleaseExecutionPendingNone,
	}
	for index, state := range wantStates {
		if got := pendingActionForConfirmedPhase(state); got != wantPending[state] {
			t.Errorf("pending action for %s = %s, want %s", state, got, wantPending[state])
		}
		for _, next := range wantStates {
			want := index+1 < len(wantStates) && next == wantStates[index+1]
			if got := state.CanTransitionTo(next); got != want {
				t.Errorf("CanTransitionTo(%s, %s) = %t, want %t", state, next, got, want)
			}
		}
	}
}

func TestDispatchJournalTransitionGraphContract(t *testing.T) {
	states := []DispatchJournalState{
		DispatchJournalPrepared,
		DispatchJournalRequestStarted,
		DispatchJournalAccepted,
		DispatchJournalRejected,
		DispatchJournalUnknown,
	}
	allowed := map[DispatchJournalState]map[DispatchJournalState]bool{
		DispatchJournalPrepared: {
			DispatchJournalRequestStarted: true,
		},
		DispatchJournalRequestStarted: {
			DispatchJournalAccepted: true,
			DispatchJournalRejected: true,
			DispatchJournalUnknown:  true,
		},
	}
	for _, state := range states {
		if !state.Valid() {
			t.Errorf("state %q is unexpectedly invalid", state)
		}
		for _, next := range states {
			want := allowed[state][next]
			if got := state.CanTransitionTo(next); got != want {
				t.Errorf("CanTransitionTo(%s, %s) = %t, want %t", state, next, got, want)
			}
		}
	}
	for _, terminal := range []DispatchJournalState{
		DispatchJournalAccepted,
		DispatchJournalRejected,
		DispatchJournalUnknown,
	} {
		for _, next := range states {
			if terminal.CanTransitionTo(next) {
				t.Errorf("terminal dispatch state %s transitions to %s", terminal, next)
			}
		}
	}
}

func TestReleaseToolIdentityAndConfigurationCandidateContract(t *testing.T) {
	tests := []struct {
		identity   string
		v1         releaseconfig.V1ReleaseSystem
		v2         releaseconfig.ExecutorType
		candidates []string
	}{
		{
			identity:   "goreleaser",
			v1:         releaseconfig.V1ReleaseTypeGoReleaser,
			v2:         releaseconfig.ExecutorGoReleaser,
			candidates: []string{".goreleaser.yml", ".goreleaser.yaml"},
		},
		{
			identity:   "jreleaser",
			v1:         releaseconfig.V1ReleaseTypeJReleaser,
			v2:         releaseconfig.ExecutorJReleaser,
			candidates: []string{"jreleaser.yml"},
		},
		{
			identity:   "release-it",
			v1:         releaseconfig.V1ReleaseTypeReleaseIt,
			v2:         releaseconfig.ExecutorReleaseIt,
			candidates: []string{".release-it.json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.identity, func(t *testing.T) {
			if string(tt.v1) != tt.identity || string(tt.v2) != tt.identity {
				t.Fatalf("schema identities differ: v1=%q v2=%q want=%q", tt.v1, tt.v2, tt.identity)
			}
			if !tt.v1.IsValid() || !tt.v2.IsValid() {
				t.Fatalf("tool identity %q is not valid in both schemas", tt.identity)
			}
			identity, err := releasetool.ParseIdentity(tt.identity)
			if err != nil {
				t.Fatalf("ParseIdentity(%q): %v", tt.identity, err)
			}
			candidates, err := releasetool.ConfigCandidates(identity)
			if err != nil {
				t.Fatalf("ConfigCandidates(%q): %v", tt.identity, err)
			}
			if !reflect.DeepEqual(candidates, tt.candidates) {
				t.Fatalf("config candidates = %v, want %v", candidates, tt.candidates)
			}
			capabilities, err := ResolveExecutorCapabilities(tt.identity)
			if err != nil {
				t.Fatalf("ResolveExecutorCapabilities(%q): %v", tt.identity, err)
			}
			if capabilities.Type != tt.identity {
				t.Fatalf("capability identity = %q, want %q", capabilities.Type, tt.identity)
			}
		})
	}

	if _, err := releasetool.ParseIdentity("semantic-release"); err == nil || !strings.Contains(err.Error(), "unknown release tool") {
		t.Fatalf("unknown tool candidate error = %v", err)
	}
}

func TestActiveV2CoordinatorRemainsDirectAndCompatibilityTransactionStaysQuarantined(t *testing.T) {
	files := parseReleaseProductionFiles(t)
	method := findReleaseMethod(t, files, "githubActionsReleaseUseCase", "Run")
	wantOperations := []string{
		"tokenResolver.ResolveGitHubActionsDispatchToken",
		"planner.Plan",
		"preflightValidator.Validate",
		"executionPreparer.Prepare",
		"materialization.Apply",
		"stateWriter.Write",
		"fileStager.Stage",
		"commitCreator.Create",
		"tagCreator.Create",
		"dispatchPreparer.Prepare",
		"commitPusher.Push",
		"tagPusher.Push",
		"workflowDispatcher.Dispatch",
		"handoffConfirmer.Confirm",
	}
	if got := orderedReceiverCapabilityCalls(method); !reflect.DeepEqual(got, wantOperations) {
		t.Fatalf("direct V2 operation order = %v, want %v", got, wantOperations)
	}
	ast.Inspect(method.Body, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			t.Errorf("authoritative V2 coordinator contains loop-driven execution")
		case *ast.FuncLit:
			t.Errorf("authoritative V2 coordinator contains callback-driven execution")
		}
		return true
	})

	assertCompatibilityTypesConfinedToDeclarations(t, files, "ReleaseTransaction", "MutationTracker")
	assertGitReleaseCoordinatorPushUnused(t, files)
}

type parsedReleaseProductionFile struct {
	path string
	file *ast.File
}

func parseReleaseProductionFiles(t *testing.T) []parsedReleaseProductionFile {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read release package: %v", err)
	}
	fileSet := token.NewFileSet()
	files := make([]parsedReleaseProductionFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(fileSet, entry.Name(), nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", entry.Name(), parseErr)
		}
		files = append(files, parsedReleaseProductionFile{path: filepath.ToSlash(entry.Name()), file: parsed})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	return files
}

func findReleaseMethod(t *testing.T, files []parsedReleaseProductionFile, receiverType, methodName string) *ast.FuncDecl {
	t.Helper()
	var match *ast.FuncDecl
	for _, parsed := range files {
		for _, declaration := range parsed.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv == nil || function.Name.Name != methodName || receiverTypeName(function.Recv.List[0].Type) != receiverType {
				continue
			}
			if match != nil {
				t.Fatalf("multiple %s.%s methods found", receiverType, methodName)
			}
			match = function
		}
	}
	if match == nil {
		t.Fatalf("%s.%s method not found", receiverType, methodName)
	}
	return match
}

func receiverTypeName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return receiverTypeName(typed.X)
	case *ast.ParenExpr:
		return receiverTypeName(typed.X)
	default:
		return ""
	}
}

func orderedReceiverCapabilityCalls(method *ast.FuncDecl) []string {
	receiverName := method.Recv.List[0].Names[0].Name
	type positionedCall struct {
		position token.Pos
		name     string
	}
	var calls []positionedCall
	ast.Inspect(method.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		methodSelector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		capabilitySelector, ok := methodSelector.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, ok := capabilitySelector.X.(*ast.Ident)
		if !ok || receiver.Name != receiverName {
			return true
		}
		calls = append(calls, positionedCall{
			position: call.Pos(),
			name:     capabilitySelector.Sel.Name + "." + methodSelector.Sel.Name,
		})
		return true
	})
	sort.Slice(calls, func(i, j int) bool { return calls[i].position < calls[j].position })
	result := make([]string, len(calls))
	for index, call := range calls {
		result[index] = call.name
	}
	return result
}

func assertCompatibilityTypesConfinedToDeclarations(t *testing.T, files []parsedReleaseProductionFile, typeNames ...string) {
	t.Helper()
	owners := make(map[string]string, len(typeNames))
	for _, parsed := range files {
		for _, declaration := range parsed.file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, specification := range general.Specs {
				typeSpecification := specification.(*ast.TypeSpec)
				for _, name := range typeNames {
					if typeSpecification.Name.Name == name {
						owners[name] = parsed.path
					}
				}
			}
		}
	}
	for _, name := range typeNames {
		owner, ok := owners[name]
		if !ok {
			t.Fatalf("compatibility type %s declaration not found", name)
		}
		for _, parsed := range files {
			if parsed.path == owner {
				continue
			}
			ast.Inspect(parsed.file, func(node ast.Node) bool {
				identifier, isIdentifier := node.(*ast.Ident)
				if isIdentifier && identifier.Name == name {
					t.Errorf("compatibility type %s is referenced by active production file %s", name, parsed.path)
				}
				return true
			})
		}
	}
}

func assertGitReleaseCoordinatorPushUnused(t *testing.T, files []parsedReleaseProductionFile) {
	t.Helper()
	for _, parsed := range files {
		for _, declaration := range parsed.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			if function.Recv != nil && function.Name.Name == "Push" && receiverTypeName(function.Recv.List[0].Type) == "GitReleaseCoordinator" {
				continue
			}
			knownCoordinators := explicitGitReleaseCoordinatorBindings(function)
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, isCall := node.(*ast.CallExpr)
				if !isCall {
					return true
				}
				selector, isSelector := call.Fun.(*ast.SelectorExpr)
				if !isSelector || selector.Sel.Name != "Push" {
					return true
				}
				if identifier, isIdentifier := selector.X.(*ast.Ident); isIdentifier && knownCoordinators[identifier.Name] {
					t.Errorf("%s calls the unjournaled GitReleaseCoordinator.Push compatibility method", parsed.path)
				}
				if expressionNamesType(selector.X, "GitReleaseCoordinator") {
					t.Errorf("%s calls the unjournaled GitReleaseCoordinator.Push compatibility method", parsed.path)
				}
				return true
			})
		}
	}
}

func explicitGitReleaseCoordinatorBindings(function *ast.FuncDecl) map[string]bool {
	bindings := map[string]bool{}
	if function.Type.Params != nil {
		for _, field := range function.Type.Params.List {
			if expressionNamesType(field.Type, "GitReleaseCoordinator") {
				for _, name := range field.Names {
					bindings[name.Name] = true
				}
			}
		}
	}
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch statement := node.(type) {
		case *ast.DeclStmt:
			general, ok := statement.Decl.(*ast.GenDecl)
			if !ok {
				return true
			}
			for _, specification := range general.Specs {
				value, ok := specification.(*ast.ValueSpec)
				if !ok || !expressionNamesType(value.Type, "GitReleaseCoordinator") {
					continue
				}
				for _, name := range value.Names {
					bindings[name.Name] = true
				}
			}
		case *ast.AssignStmt:
			for index, left := range statement.Lhs {
				identifier, ok := left.(*ast.Ident)
				if !ok || index >= len(statement.Rhs) {
					continue
				}
				if expressionCreatesGitReleaseCoordinator(statement.Rhs[index], bindings) {
					bindings[identifier.Name] = true
				}
			}
		}
		return true
	})
	return bindings
}

func expressionNamesType(expression ast.Expr, name string) bool {
	if expression == nil {
		return false
	}
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name == name
	case *ast.StarExpr:
		return expressionNamesType(typed.X, name)
	case *ast.ParenExpr:
		return expressionNamesType(typed.X, name)
	case *ast.UnaryExpr:
		return expressionNamesType(typed.X, name)
	case *ast.CompositeLit:
		return expressionNamesType(typed.Type, name)
	default:
		return false
	}
}

func expressionCreatesGitReleaseCoordinator(expression ast.Expr, bindings map[string]bool) bool {
	switch value := expression.(type) {
	case *ast.CallExpr:
		if identifier, ok := value.Fun.(*ast.Ident); ok && identifier.Name == "NewGitReleaseCoordinator" {
			return true
		}
	case *ast.Ident:
		return bindings[value.Name]
	default:
		return expressionNamesType(expression, "GitReleaseCoordinator")
	}
	return false
}
