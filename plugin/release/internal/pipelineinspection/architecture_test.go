package pipelineinspection

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

func TestPipelineInspectionHasReadOnlyDependencyReachability(t *testing.T) {
	for _, file := range pipelineProductionFiles(t) {
		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{
				"net", "net/http", "os/exec",
				"gopkg.in/yaml.v3",
				"github.com/nekoman-hq/neko-cli/internal/terminal",
				"github.com/nekoman-hq/neko-cli/pkg/log",
				"github.com/nekoman-hq/neko-cli/pkg/renderer",
				"github.com/nekoman-hq/neko-cli/plugin/release/internal/doctor",
				"github.com/nekoman-hq/neko-cli/plugin/release/internal/githubdispatch",
				"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool/goreleaser",
				"github.com/nekoman-hq/neko-cli/plugin/release/pkg/git",
				"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release",
			} {
				if path == forbidden || strings.HasPrefix(path, forbidden+"/") {
					t.Errorf("pipeline inspection imports prohibited capability %q", path)
				}
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if ok {
				for _, forbidden := range []string{"WriteFile", "Create", "Mkdir", "Chdir", "Remove", "Rename"} {
					if selector.Sel.Name == forbidden {
						t.Errorf("pipeline inspection reaches writer %s", forbidden)
					}
				}
			}
			return true
		})
	}
}

func TestPipelineInspectionDefinesNoEngineRegistryTransitionOrToolParserBehavior(t *testing.T) {
	for _, file := range pipelineProductionFiles(t) {
		for _, declaration := range file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				for _, forbidden := range []string{"Advance", "Transition", "ApplyEvent", "CanTransition", "Retry", "Resume", "ClassifyArguments", "classifiesAsRealPublication", "commaListContains"} {
					if declaration.Name.Name == forbidden {
						t.Errorf("pipeline inspection defines prohibited behavior function %s", forbidden)
					}
				}
			case *ast.GenDecl:
				for _, specification := range declaration.Specs {
					typed, ok := specification.(*ast.TypeSpec)
					if !ok {
						continue
					}
					for _, forbidden := range []string{"Engine", "Registry", "MutableContext", "Invocation"} {
						if strings.Contains(typed.Name.Name, forbidden) {
							t.Errorf("pipeline inspection defines prohibited framework type %s", typed.Name.Name)
						}
					}
				}
			}
		}
	}
}

func TestPipelineInspectionProductionPolicyHasNoRepositorySpecificExceptions(t *testing.T) {
	for _, path := range pipelineProductionPaths(t) {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"plugin-release", "plugin-ui", "release-neko-cli.yml"} {
			if strings.Contains(string(content), forbidden) {
				t.Errorf("%s contains repository-specific exception %q", path, forbidden)
			}
		}
	}
}

func pipelineProductionFiles(t *testing.T) []*ast.File {
	t.Helper()
	paths := pipelineProductionPaths(t)
	files := make([]*ast.File, 0, len(paths))
	for _, path := range paths {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files = append(files, file)
	}
	return files
}

func pipelineProductionPaths(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" && !strings.HasSuffix(entry.Name(), "_test.go") {
			paths = append(paths, entry.Name())
		}
	}
	return paths
}
