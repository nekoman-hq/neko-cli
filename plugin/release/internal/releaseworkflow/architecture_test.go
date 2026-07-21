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
	const neutralReleaseToolFacts = "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
	for _, file := range parseReleaseWorkflowProductionFiles(t) {
		for _, specification := range file.Imports {
			importPath, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				t.Fatalf("unquote import: %v", err)
			}
			if importPath == "net/http" || importPath == "os" || importPath == "os/exec" ||
				(importPath != neutralReleaseToolFacts && strings.HasPrefix(importPath, "github.com/nekoman-hq/neko-cli/plugin/release/")) {
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
