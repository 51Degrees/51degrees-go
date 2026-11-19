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

package derivedproperty

// These tests follow the key considerations of the Derived Property Element.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/derived-property-element.md#key-considerations

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// reachableStrings gives every string reachable from v, following pointers, structs,
// slices, arrays, maps and interfaces, each pointer once.
func reachableStrings(v reflect.Value, seen map[uintptr]bool, out *[]string) {
	switch v.Kind() {
	case reflect.String:
		*out = append(*out, v.String())
	case reflect.Pointer:
		if v.IsNil() || seen[v.Pointer()] {
			return
		}
		seen[v.Pointer()] = true
		reachableStrings(v.Elem(), seen, out)
	case reflect.Interface:
		if !v.IsNil() {
			reachableStrings(v.Elem(), seen, out)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			reachableStrings(v.Field(i), seen, out)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			reachableStrings(v.Index(i), seen, out)
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			reachableStrings(iter.Key(), seen, out)
			reachableStrings(iter.Value(), seen, out)
		}
	}
}

// Once Build returns, the Element holds no path to the file a script came
// from.
func TestTheElementKeepsNoPathToItsScripts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Band.yaml")
	if err := os.WriteFile(path, []byte(bandYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := NewBuilder().AddScriptFile(path).Build()
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	held := *e
	held.BaseElement = nil
	reachableStrings(reflect.ValueOf(held), map[uintptr]bool{}, &found)
	for _, s := range found {
		if strings.Contains(s, dir) {
			t.Errorf("the Element holds %q, the path of its script", s)
		}
	}
}

// locks gives the type of every mutex reachable from v.
func locks(v reflect.Value, seen map[uintptr]bool, out *[]string) {
	mutex := reflect.TypeOf(sync.Mutex{})
	rwMutex := reflect.TypeOf(sync.RWMutex{})
	if v.Type() == mutex || v.Type() == rwMutex {
		*out = append(*out, v.Type().String())
		return
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() || seen[v.Pointer()] {
			return
		}
		seen[v.Pointer()] = true
		locks(v.Elem(), seen, out)
	case reflect.Interface:
		if !v.IsNil() {
			locks(v.Elem(), seen, out)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			locks(v.Field(i), seen, out)
		}
	}
}

// The Element Data a request's values go in takes no lock to write or read.
func TestDerivedDataTakesNoLock(t *testing.T) {
	var found []string
	locks(reflect.ValueOf(newData()), map[uintptr]bool{}, &found)
	if len(found) > 0 {
		t.Errorf("each request's Element Data holds a lock: %v", found)
	}
}

// docOf gives the doc comment of the type named in the file, with its line
// breaks joined.
func docOf(t *testing.T, file, name string) string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil,
		parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, s := range g.Specs {
			if ts, ok := s.(*ast.TypeSpec); ok && ts.Name.Name == name {
				return strings.Join(strings.Fields(g.Doc.Text()), " ")
			}
		}
	}
	t.Fatalf("%s declares no type %s", file, name)
	return ""
}

// The doc comment of Data says two Derived Property Elements must not be in
// one parallel group, rather than that the Element Data is safe for several
// at once.
func TestTheDataCommentKeepsTwoElementsOutOfOneGroup(t *testing.T) {
	doc := docOf(t, "element.go", "Data")
	if !strings.Contains(doc, "MUST NOT be members of the same parallel group") {
		t.Errorf("the doc comment of Data does not say two Derived Property "+
			"Elements MUST NOT be members of the same parallel group, in:\n%s",
			doc)
	}
	if strings.Contains(doc, "concurrent") {
		t.Errorf("the doc comment of Data says it is for concurrent use, "+
			"in:\n%s", doc)
	}
}