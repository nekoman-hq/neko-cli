package release

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestCompatibilityFilesContainOnlyClassifiedCompatibilityDeclarations(t *testing.T) {
	inventory := map[string]map[string]string{
		"v1_preflight_compatibility.go": compatibilityDeclarations("legacy-forwarder",
			"Preflight", "checkV1ReleasePreflight"),
		"v1_registry_compatibility.go": compatibilityDeclarations("legacy-registry",
			"tools", "Register", "Get"),
		"v1_service_compatibility.go": compatibilityDeclarations("deprecated-wrapper",
			"Service", "NewReleaseService", "NewReleaseServiceWithContext", "Run", "GetNewVersion", "repositoryRoot"),
		"v1_tool_compatibility.go": compatibilityDeclarations("legacy-wrapper",
			"Tool", "ToolBase", "ValidateRequirements", "ResolveFiles", "InUnitRoot", "RequireBinary",
			"GitReleaseState", "hasMutatingStep", "RevertGitRelease", "DeleteGitHubRelease",
			"CreateReleaseCommit", "CreateGitTag", "PushCommits", "PushGitTag"),
		"v1_version_guard_compatibility.go": compatibilityDeclarations("deprecated-wrapper",
			"VersionGuardOptions", "refreshVersionTags", "latestVersionTag", "VersionGuard",
			"VersionGuardWithOptions", "EnsureVersionIsValid"),
		"v2_local_transaction_compatibility.go": compatibilityDeclarations("deprecated-wrapper",
			"ExecutionPhase", "ExecutionPhasePlanned", "ExecutionPhasePreflightValidated",
			"ExecutionPhaseMaterializationPrepared", "ExecutionPhaseMaterializationApplied",
			"ExecutionPhaseStatePrepared", "ExecutionPhaseReleaseFilesStaged",
			"ExecutionPhaseCommitOrTagStarted", "ExecutionPhaseRemoteSideEffectStarted",
			"ExecutionPhaseCompleted", "ExecutionPhaseFailed", "MutationTracker", "NewMutationTracker",
			"Mark", "TrackFile", "TrackStagedFile", "ReleaseTransactionResult", "transactionExecutor",
			"ReleaseTransaction", "NewReleaseTransaction", "Execute"),
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool, len(inventory))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_compatibility.go") {
			continue
		}
		expected, ok := inventory[entry.Name()]
		if !ok {
			t.Errorf("compatibility file %s has no explicit declaration classification", entry.Name())
			continue
		}
		seen[entry.Name()] = true
		for _, declaration := range topLevelDeclarationNames(t, entry.Name()) {
			category, classified := expected[declaration]
			if !classified {
				t.Errorf("%s declaration %s is not classified as compatibility code", entry.Name(), declaration)
				continue
			}
			if category == "" {
				t.Errorf("%s declaration %s has an empty compatibility category", entry.Name(), declaration)
			}
			delete(expected, declaration)
		}
		for _, missing := range sortedCompatibilityNames(expected) {
			t.Errorf("%s classified compatibility declaration %s is missing", entry.Name(), missing)
		}
	}
	for file := range inventory {
		if !seen[file] {
			t.Errorf("classified compatibility file %s is missing", file)
		}
	}
}

func TestActiveReleasePlanHasAnActiveOwner(t *testing.T) {
	declarations := topLevelDeclarationNames(t, "v2_release_plan.go")
	want := map[string]bool{
		"ReleasePlan":      true,
		"BuildReleasePlan": true,
		"ownershipSummary": true,
	}
	for _, declaration := range declarations {
		delete(want, declaration)
	}
	for missing := range want {
		t.Errorf("active V2 release-plan owner is missing %s", missing)
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_compatibility.go") {
			continue
		}
		for _, declaration := range topLevelDeclarationNames(t, entry.Name()) {
			if declaration == "ReleasePlan" || declaration == "BuildReleasePlan" || declaration == "ownershipSummary" {
				t.Errorf("active release-plan declaration %s remains in %s", declaration, entry.Name())
			}
		}
	}
}

func TestExtractedCommandRootFacadesRemainThin(t *testing.T) {
	for _, path := range []string{"doctor.go", "unit_overview.go", "workflow_init.go", "context_validation.go"} {
		parsed := parseCompatibilityArchitectureFile(t, path)
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if function.Body == nil || len(function.Body.List) != 1 {
				t.Errorf("root facade %s.%s contains implementation logic", path, function.Name.Name)
				continue
			}
			returned, ok := function.Body.List[0].(*ast.ReturnStmt)
			if !ok || len(returned.Results) != 1 {
				t.Errorf("root facade %s.%s is not one direct return", path, function.Name.Name)
				continue
			}
			call, ok := returned.Results[0].(*ast.CallExpr)
			if !ok {
				t.Errorf("root facade %s.%s does not directly call its internal owner", path, function.Name.Name)
				continue
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				t.Errorf("root facade %s.%s does not call an internal selector", path, function.Name.Name)
				continue
			}
			owner, ownerOK := selector.X.(*ast.Ident)
			if !ownerOK || !strings.Contains(owner.Name, "doctor") && owner.Name != "unitoverview" && owner.Name != "workflowinit" && owner.Name != "contextvalidation" {
				t.Errorf("root facade %s.%s does not forward to a focused internal owner", path, function.Name.Name)
			}
		}
	}
}

func TestExtractedCommandImplementationsRemainOutsideRootRelease(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		parsed := parseCompatibilityArchitectureFile(t, entry.Name())
		for _, declaration := range parsed.Decls {
			name := ""
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				name = declaration.Name.Name
			case *ast.GenDecl:
				for _, specification := range declaration.Specs {
					if typed, ok := specification.(*ast.TypeSpec); ok {
						name = typed.Name.Name
					}
				}
			}
			for _, implementationPrefix := range []string{
				"integrationDoctor", "unitOverview", "githubWorkflowScaffold",
				"releaseContextValidationUseCase", "filesystemReleaseContext",
			} {
				if strings.HasPrefix(name, implementationPrefix) {
					t.Errorf("root Release declaration %s in %s retains extracted command implementation", name, entry.Name())
				}
			}
		}
	}
}

func compatibilityDeclarations(category string, names ...string) map[string]string {
	declarations := make(map[string]string, len(names))
	for _, name := range names {
		declarations[name] = category
	}
	return declarations
}

func sortedCompatibilityNames(declarations map[string]string) []string {
	names := make([]string, 0, len(declarations))
	for name := range declarations {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func topLevelDeclarationNames(t *testing.T, path string) []string {
	t.Helper()
	parsed := parseCompatibilityArchitectureFile(t, path)
	names := make([]string, 0)
	for _, declaration := range parsed.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			names = append(names, declaration.Name.Name)
		case *ast.GenDecl:
			for _, specification := range declaration.Specs {
				switch specification := specification.(type) {
				case *ast.TypeSpec:
					names = append(names, specification.Name.Name)
				case *ast.ValueSpec:
					for _, name := range specification.Names {
						names = append(names, name.Name)
					}
				}
			}
		}
	}
	return names
}

func parseCompatibilityArchitectureFile(t *testing.T, path string) *ast.File {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return parsed
}
