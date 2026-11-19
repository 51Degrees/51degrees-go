/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2026 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
 *
 * This Original Work is licensed under the European Union Public Licence
 * (EUPL) v.1.2 and is subject to its terms as set out below.
 *
 * If a copy of the EUPL was not distributed with this file, You can obtain
 * one at https://opensource.org/licenses/EUPL-1.2.
 *
 * The 'Compatible Licences' set out in the Appendix to the EUPL (as may be
 * amended by the European Commission) shall be deemed incompatible for
 * the purposes of the Work and the provisions of the compatibility
 * clause in Article 5 of the EUPL shall not apply.
 *
 * If using the Work as, or as part of, a network application, by
 * including the attribution notice(s) required under Article 5 of the EUPL
 * in the end user terms of the application under an appropriate heading,
 * such notice(s) shall fulfill the requirements of that article.
 * ********************************************************************* */

package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// Every Flow Element in the repository has a test, in its own folder, named
// Test<Type>DataIsReadOnlyOnceProcessed, that processes it and expects a
// write to its Element Data afterwards to panic. A Flow Element is a type
// with a method Process(FlowData) error.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/thread-safety.md?plain=1#L156-L158
func TestEveryFlowElementHasAReadOnlyDataTest(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	elements := map[string][]string{} // folder to Flow Element types
	tests := map[string]bool{}        // folder and test name
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (strings.HasPrefix(name, ".") ||
				name == "testdata" || name == "vendor" ||
				name == "javascript-templates") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		dir, _ := filepath.Rel(root, filepath.Dir(path))
		dir = filepath.ToSlash(dir)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if strings.HasSuffix(name, "_test.go") {
				if fn.Recv == nil {
					tests[dir+" "+fn.Name.Name] = true
				}
				continue
			}
			if typ := flowElementType(fn); typ != "" {
				elements[dir] = append(elements[dir], typ)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(elements) == 0 {
		t.Fatal("no Flow Element was found in the repository")
	}
	for dir, types := range elements {
		for _, typ := range types {
			want := "Test" + strings.ToUpper(typ[:1]) + typ[1:] +
				"DataIsReadOnlyOnceProcessed"
			if !tests[dir+" "+want] {
				t.Errorf("%s.%s is a Flow Element with no %s in %s",
					dir, typ, want, dir)
			}
		}
	}
}

// flowElementType gives the type fn is the Process(FlowData) error method
// of, or nothing.
func flowElementType(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) != 1 || fn.Name.Name != "Process" {
		return ""
	}
	params, results := fn.Type.Params.List, fn.Type.Results
	if len(params) != 1 || len(params[0].Names) > 1 || results == nil ||
		len(results.List) != 1 {
		return ""
	}
	if id, ok := results.List[0].Type.(*ast.Ident); !ok || id.Name != "error" {
		return ""
	}
	switch p := params[0].Type.(type) {
	case *ast.Ident:
		if p.Name != "FlowData" {
			return ""
		}
	case *ast.SelectorExpr:
		if p.Sel.Name != "FlowData" {
			return ""
		}
	default:
		return ""
	}
	recv := fn.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	if id, ok := recv.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}
