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

package translation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// The doc comment of Data says two Translation Engines that write it must not
// be in one parallel group, rather than that they may write it in parallel.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/advanced-features/parallel-processing.md#how-a-group-uses-the-flow-data
func TestTheDataCommentKeepsTwoEnginesOutOfOneGroup(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "translation.go", nil,
		parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var doc string
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); ok {
			for _, s := range g.Specs {
				if ts, ok := s.(*ast.TypeSpec); ok && ts.Name.Name == "Data" {
					doc = g.Doc.Text()
				}
			}
		}
	}
	if !strings.Contains(doc, "MUST NOT be members of the same parallel group") {
		t.Errorf("the doc comment of Data does not say two Translation "+
			"Engines MUST NOT be members of the same parallel group, in:\n%s",
			doc)
	}
	if strings.Contains(doc, "in parallel") {
		t.Errorf("the doc comment of Data says Translation Engines may "+
			"write it in parallel, in:\n%s", doc)
	}
}