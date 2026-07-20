package releasetool

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestToolConfigurationPackagesRemainIndependentLeaves(t *testing.T) {
	for _, directory := range []string{"jreleaser", "releaseit"} {
		for _, parsed := range parseToolConfigurationProductionFiles(t, directory) {
			for _, specification := range parsed.Imports {
				importPath := unquoteToolConfigurationImport(t, specification.Path.Value)
				for _, forbidden := range []string{
					"github.com/nekoman-hq/neko-cli/pkg/log",
					"github.com/nekoman-hq/neko-cli/pkg/plugin",
					"github.com/nekoman-hq/neko-cli/pkg/presentation",
					"github.com/nekoman-hq/neko-cli/plugin/release/internal/doctor",
					"github.com/nekoman-hq/neko-cli/plugin/release/internal/githubdispatch",
					"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow",
					"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release",
					"net/http",
					"os/exec",
				} {
					if importPath == forbidden || strings.HasPrefix(importPath, forbidden+"/") {
						t.Errorf("%s imports forbidden dependency %q", directory, importPath)
					}
				}
			}
		}
	}
}

func TestV1ToolConfigurationFilesRemainCompatibilityFacades(t *testing.T) {
	facades := map[string]string{
		"../../pkg/release/tool/jreleaser/config.go": "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool/jreleaser",
		"../../pkg/release/tool/releaseit/config.go": "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool/releaseit",
	}
	for facadePath, ownerPath := range facades {
		parsed, err := parser.ParseFile(token.NewFileSet(), facadePath, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", facadePath, err)
		}
		ownerAliases := make(map[string]bool)
		for _, specification := range parsed.Imports {
			importPath := unquoteToolConfigurationImport(t, specification.Path.Value)
			if importPath != ownerPath {
				t.Errorf("%s imports non-owner %q", facadePath, importPath)
				continue
			}
			alias := path.Base(importPath)
			if specification.Name != nil {
				alias = specification.Name.Name
			}
			ownerAliases[alias] = true
		}
		for _, declaration := range parsed.Decls {
			switch typed := declaration.(type) {
			case *ast.GenDecl:
				if typed.Tok != token.TYPE {
					continue
				}
				for _, specification := range typed.Specs {
					typeSpecification, ok := specification.(*ast.TypeSpec)
					if !ok {
						t.Fatalf("%s contains non-type specification in type declaration", facadePath)
					}
					if !typeSpecification.Assign.IsValid() {
						t.Errorf("%s redeclares configuration type %s instead of aliasing it", facadePath, typeSpecification.Name.Name)
					}
				}
			case *ast.FuncDecl:
				calls := 0
				ast.Inspect(typed.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					calls++
					selector, selectorOK := call.Fun.(*ast.SelectorExpr)
					if !selectorOK {
						t.Errorf("%s.%s calls a non-owner implementation", facadePath, typed.Name.Name)
						return true
					}
					identifier, identifierOK := selector.X.(*ast.Ident)
					if !identifierOK || !ownerAliases[identifier.Name] {
						t.Errorf("%s.%s calls a non-owner implementation", facadePath, typed.Name.Name)
					}
					return true
				})
				if calls != 1 {
					t.Errorf("%s.%s contains %d calls, want one direct owner delegation", facadePath, typed.Name.Name, calls)
				}
			}
		}
	}
}

func TestV2JReleaserMaterializationUsesCanonicalRewriter(t *testing.T) {
	const ownerPath = "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool/jreleaser"
	found := false
	for _, parsed := range parseToolConfigurationProductionFiles(t, "../../pkg/release") {
		ownerAliases := make(map[string]bool)
		for _, specification := range parsed.Imports {
			importPath := unquoteToolConfigurationImport(t, specification.Path.Value)
			if importPath != ownerPath {
				continue
			}
			alias := path.Base(importPath)
			if specification.Name != nil {
				alias = specification.Name.Name
			}
			ownerAliases[alias] = true
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.FuncDecl:
				if typed.Name.Name == "materializeJReleaserVersion" || typed.Name.Name == "findYAMLPath" {
					t.Errorf("root Release retains JReleaser rewriting helper %s", typed.Name.Name)
				}
			case *ast.CallExpr:
				selector, ok := typed.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "RewriteProjectVersion" {
					return true
				}
				identifier, ok := selector.X.(*ast.Ident)
				if ok && ownerAliases[identifier.Name] {
					found = true
				}
			}
			return true
		})
	}
	if !found {
		t.Fatal("V2 materialization does not use canonical JReleaser version rewriting")
	}
}

func parseToolConfigurationProductionFiles(t *testing.T, directory string) []*ast.File {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read %s: %v", directory, err)
	}
	files := make([]*ast.File, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		filePath := filepath.Join(directory, entry.Name())
		parsed, parseErr := parser.ParseFile(token.NewFileSet(), filePath, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", filePath, parseErr)
		}
		files = append(files, parsed)
	}
	return files
}

func unquoteToolConfigurationImport(t *testing.T, value string) string {
	t.Helper()
	result, err := strconv.Unquote(value)
	if err != nil {
		t.Fatalf("unquote import %s: %v", value, err)
	}
	return result
}
