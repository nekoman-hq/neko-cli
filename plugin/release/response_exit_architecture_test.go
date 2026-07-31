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

func TestReleaseResponseProducersAssignOnlyExplicitZeroOrOne(t *testing.T) {
	t.Parallel()

	producerFiles := 0
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		parsed, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		responseLiterals := 0
		explicitAssignments := 0
		ast.Inspect(parsed, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.CompositeLit:
				if selector, ok := typed.Type.(*ast.SelectorExpr); ok && selector.Sel.Name == "Response" {
					if identifier, ok := selector.X.(*ast.Ident); ok && identifier.Name == "plugin" {
						responseLiterals++
					}
				}
				for _, element := range typed.Elts {
					if keyed, ok := element.(*ast.KeyValueExpr); ok {
						if key, ok := keyed.Key.(*ast.Ident); ok && key.Name == "ExitCode" {
							t.Errorf("%s assigns ExitCode directly; use presence-aware SetExitCode", path)
						}
					}
				}
			case *ast.AssignStmt:
				for _, expression := range typed.Lhs {
					if selector, ok := expression.(*ast.SelectorExpr); ok && selector.Sel.Name == "ExitCode" {
						t.Errorf("%s assigns ExitCode directly; use presence-aware SetExitCode", path)
					}
				}
			case *ast.CallExpr:
				selector, ok := typed.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "SetExitCode" {
					break
				}
				explicitAssignments++
				if len(typed.Args) != 1 {
					t.Errorf("%s has SetExitCode with %d arguments", path, len(typed.Args))
					break
				}
				literal, ok := typed.Args[0].(*ast.BasicLit)
				if !ok || literal.Kind != token.INT {
					t.Errorf("%s assigns a non-literal Release exit", path)
					break
				}
				code, conversionErr := strconv.Atoi(literal.Value)
				if conversionErr != nil || (code != 0 && code != 1) {
					t.Errorf("%s assigns Release exit %s; only 0 and 1 are allowed", path, literal.Value)
				}
			}
			return true
		})

		if responseLiterals > 0 {
			producerFiles++
			if explicitAssignments < responseLiterals {
				t.Errorf("%s creates %d plugin responses but has only %d explicit exit assignments", path, responseLiterals, explicitAssignments)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inspect Release response producers: %v", err)
	}
	if producerFiles == 0 {
		t.Fatal("no Release response producers were inspected")
	}
}
