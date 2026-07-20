package githubdispatch

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestGitHubDispatchLeafHasNoReleaseLifecycleOwnership(t *testing.T) {
	for _, file := range parseGitHubDispatchProductionFiles(t) {
		for _, specification := range file.Imports {
			importPath, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				t.Fatalf("unquote import: %v", err)
			}
			if githubDispatchImportIsProhibited(importPath) {
				t.Errorf("GitHub dispatch transport imports prohibited capability %q", importPath)
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch declaration := node.(type) {
			case *ast.TypeSpec:
				assertGitHubDispatchDeclarationIsTransportOnly(t, declaration.Name.Name)
			case *ast.FuncDecl:
				assertGitHubDispatchDeclarationIsTransportOnly(t, declaration.Name.Name)
			case *ast.BasicLit:
				if declaration.Kind == token.STRING && (declaration.Value == `"prepared"` || declaration.Value == `"request-started"`) {
					t.Errorf("transport leaf declares lifecycle outcome %s", declaration.Value)
				}
			}
			return true
		})
	}
}

func githubDispatchImportIsProhibited(importPath string) bool {
	if importPath == "os" || importPath == "os/exec" {
		return true
	}
	const repositoryImportPrefix = "github.com/nekoman-hq/neko-cli/"
	const staticWorkflowImport = repositoryImportPrefix + "plugin/release/internal/releaseworkflow"
	if strings.HasPrefix(importPath, repositoryImportPrefix) && importPath != staticWorkflowImport {
		return true
	}
	return false
}

func assertGitHubDispatchDeclarationIsTransportOnly(t *testing.T, name string) {
	t.Helper()
	for _, prohibited := range []string{"Journal", "Recovery", "Resume", "Pending", "Transition", "ExecutionContext", "Doctor"} {
		if strings.Contains(name, prohibited) {
			t.Errorf("GitHub dispatch transport declares lifecycle owner %q", name)
		}
	}
}

func TestGitHubDispatchLeafHasExactlyOneNonRetriedPOST(t *testing.T) {
	postConstructions := 0
	transportCalls := 0
	for _, file := range parseGitHubDispatchProductionFiles(t) {
		loopDepth := 0
		stack := make([]ast.Node, 0)
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				last := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				switch last.(type) {
				case *ast.ForStmt, *ast.RangeStmt:
					loopDepth--
				}
				return true
			}
			stack = append(stack, node)
			switch node.(type) {
			case *ast.ForStmt, *ast.RangeStmt:
				loopDepth++
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if selector.Sel.Name == "NewRequestWithContext" && callUsesHTTPMethodPost(call) {
				postConstructions++
			}
			if selector.Sel.Name == "Do" {
				transportCalls++
				if loopDepth != 0 {
					t.Error("workflow-dispatch transport call is nested in a retry-capable loop")
				}
			}
			return true
		})
	}
	if postConstructions != 1 || transportCalls != 1 {
		t.Fatalf("POST constructions=%d transport calls=%d, want exactly one each", postConstructions, transportCalls)
	}
}

func callUsesHTTPMethodPost(call *ast.CallExpr) bool {
	if len(call.Args) < 2 {
		return false
	}
	selector, ok := call.Args[1].(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "MethodPost" {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	return ok && identifier.Name == "http"
}

func parseGitHubDispatchProductionFiles(t *testing.T) []*ast.File {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package: %v", err)
	}
	set := token.NewFileSet()
	files := make([]*ast.File, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(set, entry.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", entry.Name(), parseErr)
		}
		files = append(files, file)
	}
	return files
}
