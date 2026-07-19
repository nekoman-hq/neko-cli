package presentation_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalProductionCodeDoesNotUseDeprecatedPresentationNames(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	allowed := map[string]map[string]bool{
		"pkg/plugin/presentation_compatibility.go": {"*": true},
		"pkg/plugin/response_presentation.go": {
			"HumanTable": true, "HumanProperties": true, "HumanText": true,
		},
		"pkg/plugin/types.go": {
			"HumanTable": true, "HumanProperties": true, "HumanText": true,
		},
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if strings.Contains(filepath.Base(path), "human_") {
			t.Errorf("canonical production filename uses deprecated presentation vocabulary: %s", relative)
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			identifier, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			pathAllowlist := allowed[filepath.ToSlash(relative)]
			identifierAllowed := pathAllowlist["*"] || pathAllowlist[identifier.Name]
			if strings.HasPrefix(identifier.Name, "Human") && !identifierAllowed {
				t.Errorf("canonical production identifier %s uses deprecated presentation vocabulary in %s", identifier.Name, relative)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("inspect repository naming: %v", err)
	}
}

func TestLegacyPresentationWireTagsAreIsolated(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(contents), "human_") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if filepath.ToSlash(relative) != "pkg/plugin/response_presentation.go" {
			t.Errorf("legacy presentation wire name is not isolated in the protocol adapter: %s", relative)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inspect presentation wire tags: %v", err)
	}
}

func TestPresentationDeclarationsDoNotDependOnRendering(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read presentation package: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}
		for _, imported := range file.Imports {
			if strings.Contains(imported.Path.Value, "/renderer") {
				t.Fatalf("presentation declaration file %s imports renderer", entry.Name())
			}
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}
