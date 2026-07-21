package goreleaser

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

func TestGoReleaserFactsRemainNeutralAndInfrastructureFree(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package: %v", err)
	}
	forbiddenImports := []string{
		"github.com/nekoman-hq/neko-cli/internal/terminal",
		"github.com/nekoman-hq/neko-cli/pkg/log",
		"github.com/nekoman-hq/neko-cli/pkg/plugin",
		"github.com/nekoman-hq/neko-cli/pkg/presentation",
		"github.com/nekoman-hq/neko-cli/plugin/release/internal/doctor",
		"github.com/nekoman-hq/neko-cli/plugin/release/internal/githubdispatch",
		"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow",
		"github.com/nekoman-hq/neko-cli/plugin/release/pkg/git",
		"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release",
		"io/fs",
		"net/http",
		"os",
		"os/exec",
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", entry.Name(), parseErr)
		}
		for _, imported := range parsed.Imports {
			path, unquoteErr := strconv.Unquote(imported.Path.Value)
			if unquoteErr != nil {
				t.Fatalf("unquote import in %s: %v", entry.Name(), unquoteErr)
			}
			for _, forbidden := range forbiddenImports {
				if path == forbidden || strings.HasPrefix(path, forbidden+"/") {
					t.Errorf("%s imports forbidden infrastructure %q", entry.Name(), path)
				}
			}
		}
		for _, declaration := range parsed.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, specification := range general.Specs {
				typeSpecification, ok := specification.(*ast.TypeSpec)
				if !ok {
					t.Fatalf("%s contains non-type specification in type declaration", entry.Name())
				}
				name := typeSpecification.Name.Name
				for _, forbidden := range []string{"Diagnostic", "Severity", "Remediation", "Readiness"} {
					if strings.Contains(name, forbidden) {
						t.Errorf("%s declares Doctor-owned type %q", entry.Name(), name)
					}
				}
			}
		}
	}
}

func TestGoReleaserPackageOwnsInvocationClassification(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "invocation.go", nil, 0)
	if err != nil {
		t.Fatalf("parse canonical GoReleaser invocation owner: %v", err)
	}
	foundType := false
	foundFunctions := map[string]bool{
		"ClassifyArguments":           false,
		"classifiesAsRealPublication": false,
		"commaListContains":           false,
	}
	for _, declaration := range parsed.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			if _, required := foundFunctions[typed.Name.Name]; required {
				foundFunctions[typed.Name.Name] = true
			}
		case *ast.GenDecl:
			if typed.Tok != token.TYPE {
				continue
			}
			for _, specification := range typed.Specs {
				typeSpecification, ok := specification.(*ast.TypeSpec)
				if ok && typeSpecification.Name.Name == "Invocation" {
					foundType = true
				}
			}
		}
	}
	if !foundType {
		t.Error("GoReleaser package does not declare Invocation")
	}
	for name, found := range foundFunctions {
		if !found {
			t.Errorf("GoReleaser package does not declare %s", name)
		}
	}
}
