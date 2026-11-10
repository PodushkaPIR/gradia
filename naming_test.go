package gradia_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

const maxFunctionName = 35

func TestFunctionNameLimit(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Local notes and vendored code are not maintained application sources.
		if entry.IsDir() {
			if path != "." && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "docs" || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		violations, err := findLongFuncNames(path, nil)
		if err != nil {
			return err
		}
		for _, violation := range violations {
			t.Error(violation)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func findLongFuncNames(path string, source any) ([]string, error) {
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, path, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var violations []string
	check := func(name *ast.Ident) {
		if length := utf8.RuneCountInString(name.Name); length > maxFunctionName {
			violations = append(violations, fmt.Sprintf(
				"%s: %s has %d characters; maximum is %d",
				positions.Position(name.Pos()), name.Name, length, maxFunctionName,
			))
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncDecl:
			check(node.Name)
		case *ast.InterfaceType:
			for _, method := range node.Methods.List {
				for _, name := range method.Names {
					check(name)
				}
			}
		}
		return true
	})
	return violations, nil
}

func TestNameLimitChecker(t *testing.T) {
	for _, test := range []struct {
		name    string
		pattern string
		length  int
		want    int
	}{
		{"below limit", "func %s() {}", 34, 0},
		{"at limit", "func %s() {}", 35, 0},
		{"above limit", "func %s() {}", 36, 1},
		{"method", "func (x X) %s() {}", 36, 1},
		{"interface method", "type X interface { %s() }", 36, 1},
		{"unicode at limit", "func %s() {}", 35, 0},
		{"unicode above limit", "func %s() {}", 36, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			letter := "A"
			if strings.HasPrefix(test.name, "unicode") {
				letter = "Я"
			}
			source := "package sample\n" + fmt.Sprintf(test.pattern, strings.Repeat(letter, test.length))
			violations, err := findLongFuncNames("sample.go", source)
			if err != nil || len(violations) != test.want {
				t.Fatalf("violations = %v, error = %v; want %d violations", violations, err, test.want)
			}
		})
	}
	if _, err := findLongFuncNames("invalid.go", "not Go code"); err == nil {
		t.Fatal("invalid source accepted")
	}
}
