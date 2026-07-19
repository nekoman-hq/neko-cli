package terminal

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/nekoman-hq/neko-cli"

func TestRootPublicPackagesDocumentTheirIntent(t *testing.T) {
	t.Parallel()

	root := architectureRepositoryRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "pkg"))
	if err != nil {
		t.Fatalf("read public package root: %v", err)
	}
	policy, err := os.ReadFile(filepath.Join(root, "docs", "package-architecture.md"))
	if err != nil {
		t.Fatalf("read package architecture policy: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		packageDir := filepath.Join(root, "pkg", entry.Name())
		packageName, documented := packageDocumentation(t, packageDir)
		if !documented {
			t.Errorf("pkg/%s has no package comment documenting its public intent", entry.Name())
		}
		if packageName == "" {
			t.Errorf("pkg/%s has no production Go package", entry.Name())
		}
		if !strings.Contains(string(policy), "`"+entry.Name()+"`") {
			t.Errorf("pkg/%s is not classified in docs/package-architecture.md", entry.Name())
		}
	}
}

func TestPublicPackagesUseOnlyFocusedPrivateTerminalPrimitives(t *testing.T) {
	t.Parallel()

	root := architectureRepositoryRoot(t)
	wantImport := modulePath + "/internal/terminal"
	users := map[string]bool{"log": false, "renderer": false}
	err := filepath.WalkDir(filepath.Join(root, "pkg"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, specification := range file.Imports {
			importPath, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				return err
			}
			if !strings.Contains(importPath, "/internal/") {
				continue
			}
			relative, err := filepath.Rel(filepath.Join(root, "pkg"), path)
			if err != nil {
				return err
			}
			packageDir := strings.Split(filepath.ToSlash(relative), "/")[0]
			if importPath != wantImport || (packageDir != "log" && packageDir != "renderer") {
				t.Errorf("public package file %s imports unsupported private implementation %s", relative, importPath)
				continue
			}
			users[packageDir] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inspect public package imports: %v", err)
	}
	for packageName, found := range users {
		if !found {
			t.Errorf("pkg/%s no longer shares the focused private terminal primitives", packageName)
		}
	}
}

func TestRendererAndLoggerRemainIndependent(t *testing.T) {
	t.Parallel()

	root := architectureRepositoryRoot(t)
	assertPackageDoesNotImport(t, filepath.Join(root, "pkg", "renderer"), modulePath+"/pkg/log")
	assertPackageDoesNotImport(t, filepath.Join(root, "pkg", "log"), modulePath+"/pkg/renderer")
}

func packageDocumentation(t *testing.T, packageDir string) (string, bool) {
	t.Helper()
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		t.Fatalf("read %s: %v", packageDir, err)
	}
	packageName := ""
	documented := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(packageDir, entry.Name()), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}
		packageName = file.Name.Name
		if file.Doc != nil && strings.HasPrefix(file.Doc.Text(), "Package "+packageName+" ") {
			documented = true
		}
	}
	return packageName, documented
}

func assertPackageDoesNotImport(t *testing.T, packageDir, forbidden string) {
	t.Helper()
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		t.Fatalf("read package %s: %v", packageDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(packageDir, entry.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}
		for _, specification := range file.Imports {
			path, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", entry.Name(), err)
			}
			if path == forbidden {
				t.Errorf("%s imports forbidden package %s", entry.Name(), forbidden)
			}
		}
	}
}

func architectureRepositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}
