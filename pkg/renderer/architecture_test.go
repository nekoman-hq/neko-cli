package renderer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestRendererDoesNotDependOnLogger(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read renderer package: %v", err)
	}
	loggerImport := strings.Join([]string{"github.com/nekoman-hq/neko-cli/pkg", "log"}, "/")
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", entry.Name(), parseErr)
		}
		for _, declaration := range file.Decls {
			importDeclaration, ok := declaration.(*ast.GenDecl)
			if !ok || importDeclaration.Tok != token.IMPORT {
				continue
			}
			for _, specification := range importDeclaration.Specs {
				importSpecification, ok := specification.(*ast.ImportSpec)
				if !ok {
					t.Fatalf("unexpected non-import specification in %s", entry.Name())
				}
				path, unquoteErr := strconv.Unquote(importSpecification.Path.Value)
				if unquoteErr != nil {
					t.Fatalf("unquote import in %s: %v", entry.Name(), unquoteErr)
				}
				if path == loggerImport {
					t.Fatalf("renderer file %s depends on logger", entry.Name())
				}
			}
		}
	}
}
