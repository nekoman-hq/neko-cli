package releaseworkflow

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

func TestReleaseWorkflowFactsHaveNoHTTPCapabilityOrLifecycleOwnership(t *testing.T) {
	const focusedGoReleaserFacts = "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool/goreleaser"
	for _, file := range parseReleaseWorkflowProductionFiles(t) {
		for _, specification := range file.Imports {
			importPath, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				t.Fatalf("unquote import: %v", err)
			}
			if importPath == "net/http" || importPath == "os" || importPath == "os/exec" ||
				(importPath != focusedGoReleaserFacts && strings.HasPrefix(importPath, "github.com/nekoman-hq/neko-cli/plugin/release/")) {
				t.Errorf("static release workflow facts import prohibited capability %q", importPath)
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch declaration := node.(type) {
			case *ast.TypeSpec:
				assertReleaseWorkflowDeclarationIsStatic(t, declaration.Name.Name)
			case *ast.FuncDecl:
				assertReleaseWorkflowDeclarationIsStatic(t, declaration.Name.Name)
			}
			return true
		})
	}
}

func TestReleaseWorkflowUsesFocusedGoReleaserInvocationFacts(t *testing.T) {
	const focusedGoReleaserFacts = "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool/goreleaser"
	foundImport, foundCall := false, false
	for _, file := range parseReleaseWorkflowProductionFiles(t) {
		aliases := make(map[string]bool)
		for _, specification := range file.Imports {
			importPath, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				t.Fatalf("unquote import: %v", err)
			}
			if importPath != focusedGoReleaserFacts {
				continue
			}
			foundImport = true
			alias := filepath.Base(importPath)
			if specification.Name != nil {
				alias = specification.Name.Name
			}
			aliases[alias] = true
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "ClassifyArguments" {
				return true
			}
			owner, ok := selector.X.(*ast.Ident)
			if ok && aliases[owner.Name] {
				foundCall = true
			}
			return true
		})
	}
	if !foundImport || !foundCall {
		t.Fatalf("release workflow does not consume focused goreleaser.ClassifyArguments: import=%t call=%t", foundImport, foundCall)
	}
}

func assertReleaseWorkflowDeclarationIsStatic(t *testing.T, name string) {
	t.Helper()
	for _, prohibited := range []string{"Token", "Journal", "Transition", "Recovery", "Resume", "Lifecycle"} {
		if strings.Contains(name, prohibited) {
			t.Errorf("static release workflow facts declare lifecycle or credential owner %q", name)
		}
	}
}

func parseReleaseWorkflowProductionFiles(t *testing.T) []*ast.File {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package: %v", err)
	}
	files := make([]*ast.File, 0, len(entries))
	set := token.NewFileSet()
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
