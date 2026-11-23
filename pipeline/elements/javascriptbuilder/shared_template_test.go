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

package javascriptbuilder

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/jsonbuilder"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/sequence"
)

// repositoryRoot is the root of the repository, relative to this package's
// folder.
const repositoryRoot = "../../../"

// sharedTemplatePath is JavaScriptResource.mustache in the
// javascript-templates submodule.
const sharedTemplatePath = repositoryRoot +
	"javascript-templates/JavaScriptResource.mustache"

// readSharedTemplate returns the template in the javascript-templates
// submodule. A test run from the module cache has no submodule, so it is
// skipped.
func readSharedTemplate(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(repositoryRoot + ".gitmodules"); err != nil {
		t.Skip("not run from a clone of the repository, so there is no " +
			"javascript-templates submodule")
	}
	b, err := os.ReadFile(sharedTemplatePath)
	if err != nil {
		t.Fatalf("%v, so run git submodule update --init "+
			"javascript-templates", err)
	}
	return string(b)
}

// lf gives s with LF line endings, because a Windows checkout can give the
// template CRLF ones.
func lf(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

// firstDifference describes the first line where got and want differ.
func firstDifference(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(g) && i < len(w); i++ {
		if g[i] != w[i] {
			return fmt.Sprintf("line %d is %q, want %q", i+1, g[i], w[i])
		}
	}
	return fmt.Sprintf("%d lines, want %d", len(g), len(w))
}

// The element renders the template that the process section of the
// specification says every implementation uses, javascript-templates'
// JavaScriptResource.mustache.
func TestTemplateIsTheSharedTemplate(t *testing.T) {
	shared := lf(readSharedTemplate(t))
	if embedded := lf(templateSource); embedded != shared {
		t.Fatalf("the embedded template is not %s, %s, so copy the "+
			"submodule's file into this package's javascript-templates "+
			"folder", sharedTemplatePath, firstDifference(embedded, shared))
	}
}

// For the same inputs, the script is the shared template rendered with the
// values the process section lists, and with _supportsFetch and
// _userPrompt, which the template also reads.
func TestScriptIsTheSharedTemplateRendered(t *testing.T) {
	shared := readSharedTemplate(t)
	const sessionID = "12345678-1234-4234-8234-1234567890ab"
	for _, c := range []struct {
		name     string
		evidence map[string]string
		want     map[string]any
	}{
		{"callback", map[string]string{
			"query.session-id": sessionID,
			"header.protocol":  "https",
			"header.host":      "example.com",
			"query.myparam":    "my value",
		}, map[string]any{
			"_objName":              "fod",
			"_sessionId":            sessionID,
			"_sequence":             1,
			"_parameters":           `{"myparam":"my+value"}`,
			"_url":                  "https://example.com/51dpipeline/json",
			"_updateEnabled":        true,
			"_enableCookies":        true,
			"_hasDelayedProperties": false,
			"_supportsPromises":     false,
			"_supportsFetch":        false,
			"_userPrompt":           false,
		}},
		{"no callback", map[string]string{
			"query.session-id":            sessionID,
			"query.fod-js-object-name":    "myFod",
			"query.fod-js-enable-cookies": "false",
		}, map[string]any{
			"_objName":   "myFod",
			"_sessionId": sessionID,
			"_sequence":  1,
			"_parameters": `{"fod-js-enable-cookies":"false",` +
				`"fod-js-object-name":"myFod"}`,
			"_url":                  "",
			"_updateEnabled":        false,
			"_enableCookies":        false,
			"_hasDelayedProperties": false,
			"_supportsPromises":     false,
			"_supportsFetch":        false,
			"_userPrompt":           false,
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			p, err := core.NewPipelineBuilder().
				AddFlowElement(sequence.New(nil)).
				AddFlowElement(jsonbuilder.New(nil, nil)).
				AddFlowElement(NewBuilder().SetEndpoint(DefaultEndpoint).
					SetMinify(false).Build()).
				Build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			defer p.Close()
			fd := p.CreateFlowData()
			for k, v := range c.evidence {
				fd.AddEvidence(k, v)
			}
			if err := fd.Process(); err != nil {
				t.Fatalf("process: %v", err)
			}
			jsonData, err := fd.Get(jsonbuilder.DataKey)
			if err != nil {
				t.Fatalf("get JSON: %v", err)
			}
			c.want["_jsonObject"], err = jsonData.Get(jsonbuilder.JSONProperty)
			if err != nil {
				t.Fatalf("get JSON: %v", err)
			}
			data, err := fd.Get(DataKey)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			got := lf(data.(*Data).JavaScript())
			want := lf(parseMustache(shared).render(c.want))
			if got != want {
				t.Errorf("the script is not the shared template rendered, %s",
					firstDifference(got, want))
			}
		})
	}
}

// The checks javascript-templates' own tests make of a rendered script,
// which each language's tests make too. The script ends with the line that
// creates the object, has no Mustache tag left and no line ending in a space
// or tab, and reads document.cookie once with cookies on and never with them
// off.
func TestRenderedScriptFormatting(t *testing.T) {
	endsWithObject := regexp.MustCompile(
		`var fod = new fiftyoneDegreesManager\(\);\n?$`)
	tag := regexp.MustCompile(`\{\{[#^/!&{]?_`)
	for _, c := range []struct {
		name     string
		evidence map[string]string
		cookies  int
	}{
		{"cookies on", map[string]string{"header.host": "example.com"}, 1},
		{"cookies off", map[string]string{
			"header.host":                 "example.com",
			"query.fod-js-enable-cookies": "false",
		}, 0},
		{"no callback", map[string]string{}, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			js := lf(renderElement(t, NewBuilder().SetEndpoint(DefaultEndpoint).
				SetMinify(false).Build(), c.evidence))
			if !endsWithObject.MatchString(js) {
				t.Errorf("the script does not end with the line that "+
					"creates the object, %q", js[max(0, len(js)-80):])
			}
			if loc := tag.FindStringIndex(js); loc != nil {
				t.Errorf("a Mustache tag is left in the script, %q",
					js[loc[0]:min(len(js), loc[0]+40)])
			}
			for i, line := range strings.Split(js, "\n") {
				if strings.TrimRight(line, " \t") != line {
					t.Errorf("line %d ends in a space or tab, %q", i+1, line)
					break
				}
			}
			if n := strings.Count(js, "document.cookie"); n != c.cookies {
				t.Errorf("document.cookie appears %d times, want %d", n,
					c.cookies)
			}
		})
	}
}
