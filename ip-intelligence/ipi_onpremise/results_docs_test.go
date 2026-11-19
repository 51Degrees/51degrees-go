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

package ipi_onpremise

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// notSharedWords and callerWords are what every comment and document about
// getting a results instance says.
const (
	notSharedWords = "not safe to use from several goroutines at once"
	callerWords    = "caller is responsible for thread-safe access"
)

// funcDoc gives the doc comment, on one line, of the function name in the Go
// file at path, or of the method name of type recv when recv is not empty.
func funcDoc(t *testing.T, path, recv, name string) string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil,
		parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name {
			continue
		}
		got := ""
		if fn.Recv != nil && len(fn.Recv.List) == 1 {
			typ := fn.Recv.List[0].Type
			if star, ok := typ.(*ast.StarExpr); ok {
				typ = star.X
			}
			if id, ok := typ.(*ast.Ident); ok {
				got = id.Name
			}
		}
		if got == recv {
			return strings.Join(strings.Fields(fn.Doc.Text()), " ")
		}
	}
	t.Fatalf("%s has no function %q with receiver %q", path, name, recv)
	return ""
}

// Every public way of getting a IP Intelligence results instance says that the instance
// is not safe to use from several goroutines at once and that the caller is
// responsible for thread-safe access, and so does the module's README.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L219-L221
func TestEveryWayToGetAResultsInstanceSaysItIsNotShared(t *testing.T) {
	for _, c := range []struct{ path, recv, name string }{
		{"../ipi_interop/results_ipi.go", "", "NewResultsIpi"},
		{"onpremise.go", "Engine", "NewResultsIpi"},
		{"onpremise.go", "Engine", "ProcessWithResults"},
	} {
		doc := funcDoc(t, c.path, c.recv, c.name)
		for _, want := range []string{notSharedWords, callerWords} {
			if !strings.Contains(doc, want) {
				t.Errorf("the doc comment of %s %s does not say %q, in: %s",
					c.recv, c.name, want, doc)
			}
		}
	}
	readme, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(readme)), " ")
	for _, want := range []string{notSharedWords, callerWords} {
		if !strings.Contains(text, want) {
			t.Errorf("the README does not say %q", want)
		}
	}
}

// typeDoc gives the doc comment, on one line, of the type name in the Go file
// at path.
func typeDoc(t *testing.T, path, name string) string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil,
		parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, s := range gd.Specs {
			ts := s.(*ast.TypeSpec)
			if ts.Name.Name != name {
				continue
			}
			doc := ts.Doc
			if doc == nil {
				doc = gd.Doc
			}
			return strings.Join(strings.Fields(doc.Text()), " ")
		}
	}
	t.Fatalf("%s has no type %q", path, name)
	return ""
}

// Data reads its values from a results instance, so ProcessData, which gives
// it, the Data type and the README's thread safety section say the same of
// it as of the results instance.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L219-L221
func TestDataThatReadsAResultsInstanceSaysItIsNotShared(t *testing.T) {
	docs := map[string]string{
		"Engine ProcessData": funcDoc(t, "data.go", "Engine", "ProcessData"),
		"type Data":          typeDoc(t, "data.go", "Data"),
	}
	for name, doc := range docs {
		for _, want := range []string{notSharedWords, callerWords} {
			if !strings.Contains(doc, want) {
				t.Errorf("the doc comment of %s does not say %q, in: %s",
					name, want, doc)
			}
		}
	}
	readme, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(readme)
	start := strings.Index(text, "## Thread safety")
	if start < 0 {
		t.Fatal("the README has no Thread safety section")
	}
	section := text[start+len("## Thread safety"):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	if !strings.Contains(section, "`Engine.ProcessData`") {
		t.Errorf("the README's Thread safety section does not name "+
			"`Engine.ProcessData`, in: %s", section)
	}
}
