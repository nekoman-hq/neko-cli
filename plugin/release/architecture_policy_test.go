package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const releaseOrchestrationImport = "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"

func TestRootPluginProductionRemainsCompositionOnly(t *testing.T) {
	files := parseReleaseProductionFiles(t, ".", false)
	allowedFunctions := map[string]bool{
		"main":            true,
		"handleRequestAt": true,
	}

	for _, file := range files {
		if file.syntax.Name.Name != "main" {
			t.Errorf("root production file %s uses package %s, want main", file.path, file.syntax.Name.Name)
		}
		for _, declaration := range file.syntax.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if !allowedFunctions[declaration.Name.Name] {
					t.Errorf("root production function %s in %s exceeds the documented composition responsibility", declaration.Name.Name, file.path)
				}
			case *ast.GenDecl:
				if declaration.Tok != token.IMPORT {
					t.Errorf("root production declaration %s in %s exceeds the documented composition responsibility", declaration.Tok, file.path)
				}
			}
		}
	}
}

func TestExtractedCapabilitiesDoNotImportRootReleaseOrchestration(t *testing.T) {
	for _, file := range parseReleaseProductionFiles(t, "internal", true) {
		for _, imported := range releaseArchitectureImports(t, file) {
			if imported == releaseOrchestrationImport {
				t.Errorf("extracted capability %s imports root release orchestration", file.path)
			}
		}
	}
}

func TestReleaseToolFactsRemainInfrastructureFree(t *testing.T) {
	forbidden := []string{
		"net/http",
		"github.com/nekoman-hq/neko-cli/pkg/plugin",
		"github.com/nekoman-hq/neko-cli/plugin/release/internal/doctor",
		"github.com/nekoman-hq/neko-cli/plugin/release/internal/githubdispatch",
		"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow",
		"github.com/nekoman-hq/neko-cli/plugin/release/pkg/evidence",
		"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release",
	}

	for _, file := range parseReleaseProductionFiles(t, "internal/releasetool", true) {
		for _, imported := range releaseArchitectureImports(t, file) {
			for _, prohibited := range forbidden {
				if imported == prohibited || strings.HasPrefix(imported, prohibited+"/") {
					t.Errorf("tool fact file %s imports prohibited infrastructure %s", file.path, imported)
				}
			}
		}
	}
}

func TestReleaseProductionIntroducesNoGenericPipelineFramework(t *testing.T) {
	forbidden := map[string]bool{
		"ReleasePipeline":        true,
		"PipelineInspection":     true,
		"PipelineManager":        true,
		"ReleaseEngine":          true,
		"ReleaseStageRegistry":   true,
		"ReleaseStepRegistry":    true,
		"ReleaseMiddlewareChain": true,
	}

	for _, file := range parseReleaseProductionFiles(t, ".", true) {
		ast.Inspect(file.syntax, func(node ast.Node) bool {
			identifier, ok := node.(*ast.Ident)
			if ok && forbidden[identifier.Name] {
				t.Errorf("production file %s introduces prohibited generic lifecycle identifier %s", file.path, identifier.Name)
			}
			return true
		})
	}
}

func TestReleaseProductionPackagesHavePackageComments(t *testing.T) {
	packages := make(map[string]bool)
	for _, file := range parseReleaseProductionFiles(t, ".", true) {
		directory := filepath.Dir(file.path)
		packages[directory] = packages[directory] || file.syntax.Doc != nil
	}
	for directory, documented := range packages {
		if !documented {
			t.Errorf("production package %s has no package comment", directory)
		}
	}
}

type releaseProductionFile struct {
	syntax *ast.File
	path   string
}

func parseReleaseProductionFiles(t *testing.T, root string, recursive bool) []releaseProductionFile {
	t.Helper()
	files := make([]releaseProductionFile, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && !recursive {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		syntax, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if parseErr != nil {
			return parseErr
		}
		files = append(files, releaseProductionFile{path: path, syntax: syntax})
		return nil
	})
	if err != nil {
		t.Fatalf("parse release production files below %s: %v", root, err)
	}
	return files
}

func releaseArchitectureImports(t *testing.T, file releaseProductionFile) []string {
	t.Helper()
	imports := make([]string, 0, len(file.syntax.Imports))
	for _, specification := range file.syntax.Imports {
		imported, err := strconv.Unquote(specification.Path.Value)
		if err != nil {
			t.Fatalf("unquote import in %s: %v", file.path, err)
		}
		imports = append(imports, imported)
	}
	return imports
}
