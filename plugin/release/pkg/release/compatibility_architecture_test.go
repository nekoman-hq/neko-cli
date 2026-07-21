package release

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCompatibilityFilesContainOnlyClassifiedCompatibilityDeclarations(t *testing.T) {
	files := compatibilityProductionFiles(t)
	if len(files) == 0 {
		t.Fatal("no Release Plugin compatibility production files found")
	}
	for _, path := range files {
		parsed := parseCompatibilityArchitectureFile(t, path)
		classifiedReceiverTypes := classifiedCompatibilityReceiverTypes(parsed)
		for _, declaration := range parsed.Decls {
			if compatibilityDeclarationClassified(declaration, classifiedReceiverTypes) {
				continue
			}
			for _, name := range declarationNames(declaration) {
				t.Errorf("%s declaration %s lacks a Legacy, Deprecated, Alias, Wrapper, Forwarding, or Compatibility classification", path, name)
			}
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

	for _, path := range compatibilityProductionFiles(t) {
		for _, declaration := range topLevelDeclarationNames(t, path) {
			if declaration == "ReleasePlan" || declaration == "BuildReleasePlan" || declaration == "ownershipSummary" {
				t.Errorf("active release-plan declaration %s remains in %s", declaration, path)
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
			if !ownerOK || !strings.Contains(owner.Name, "doctor") && owner.Name != "unitoverview" && owner.Name != "workflowinit" && owner.Name != "contextvalidation" && owner.Name != "pipelineinspection" {
				t.Errorf("root facade %s.%s does not forward to a focused internal owner", path, function.Name.Name)
			}
		}
	}
}

func TestPipelineRootFacadeContainsOnlyRuntimeInspectionComposition(t *testing.T) {
	parsed := parseCompatibilityArchitectureFile(t, "pipeline.go")
	allowed := map[string]bool{
		"ResolveInspectionRepositoryRoot":  true,
		"HandlePipelineAt":                 true,
		"inspectPipelineRuntime":           true,
		"configuredReleaseLifecycleStages": true,
		"HandlePipelineRuntimeAt":          true,
		"Path":                             true,
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := ""
		switch function := call.Fun.(type) {
		case *ast.Ident:
			name = function.Name
		case *ast.SelectorExpr:
			name = function.Sel.Name
		}
		if name != "" && !allowed[name] {
			t.Errorf("pipeline root facade calls non-composition capability %s", name)
		}
		return true
	})
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

func compatibilityProductionFiles(t *testing.T) []string {
	t.Helper()
	root := filepath.Clean("..")
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "_compatibility.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk Release Plugin compatibility files: %v", err)
	}
	sort.Strings(files)
	return files
}

func classifiedCompatibilityReceiverTypes(parsed *ast.File) map[string]bool {
	types := make(map[string]bool)
	for _, declaration := range parsed.Decls {
		generated, ok := declaration.(*ast.GenDecl)
		if !ok || generated.Tok != token.TYPE || !hasCompatibilityClassification(generated.Doc) {
			continue
		}
		for _, specification := range generated.Specs {
			if typed, ok := specification.(*ast.TypeSpec); ok {
				types[typed.Name.Name] = true
			}
		}
	}
	return types
}

func compatibilityDeclarationClassified(declaration ast.Decl, receiverTypes map[string]bool) bool {
	switch declaration := declaration.(type) {
	case *ast.FuncDecl:
		return hasCompatibilityClassification(declaration.Doc) || receiverTypes[compatibilityReceiverTypeName(declaration)]
	case *ast.GenDecl:
		return declaration.Tok == token.IMPORT || hasCompatibilityClassification(declaration.Doc)
	default:
		return false
	}
}

func hasCompatibilityClassification(comments *ast.CommentGroup) bool {
	if comments == nil {
		return false
	}
	for _, marker := range []string{"Legacy:", "Deprecated:", "Alias:", "Wrapper:", "Forwarding:", "Compatibility:"} {
		if strings.Contains(comments.Text(), marker) {
			return true
		}
	}
	return false
}

func compatibilityReceiverTypeName(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) != 1 {
		return ""
	}
	switch receiver := function.Recv.List[0].Type.(type) {
	case *ast.Ident:
		return receiver.Name
	case *ast.StarExpr:
		if identifier, ok := receiver.X.(*ast.Ident); ok {
			return identifier.Name
		}
	}
	return ""
}

func declarationNames(declaration ast.Decl) []string {
	switch declaration := declaration.(type) {
	case *ast.FuncDecl:
		return []string{declaration.Name.Name}
	case *ast.GenDecl:
		var names []string
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
		return names
	default:
		return []string{"<unknown>"}
	}
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
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return parsed
}
