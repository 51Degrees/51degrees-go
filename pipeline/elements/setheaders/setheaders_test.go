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

package setheaders

import (
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// fakeEngine produces SetHeader-prefixed Properties for the Set Headers Element
// to consume.
type fakeEngine struct {
	*core.BaseElement
	values map[string]string
}

func newFakeEngine(values map[string]string) *fakeEngine {
	e := &fakeEngine{BaseElement: core.NewBaseElement("device", core.NewWhitelistFilter(), nil), values: values}
	props := make([]core.PropertyMetadata, 0, len(values))
	for name := range values {
		props = append(props, core.PropertyMetadata{Name: name, Element: e, Available: true})
	}
	e.SetProperties(props)
	return e
}

func (e *fakeEngine) Process(fd core.FlowData) error {
	data := fd.GetOrAdd("device", func(core.FlowData) core.ElementData { return core.NewBaseElementData() })
	for name, value := range e.values {
		data.Set(name, value)
	}
	return nil
}

func TestSetHeaders(t *testing.T) {
	src := newFakeEngine(map[string]string{
		"SetHeaderBrowserAccept-CH":  "Sec-CH-UA,Sec-CH-UA-Mobile",
		"SetHeaderHardwareAccept-CH": "Sec-CH-UA-Model,Sec-CH-UA-Mobile",
		"SetHeaderUnknownAccept-CH":  "Unknown",
	})
	sh := New(nil)
	p, err := core.NewPipelineBuilder().AddFlowElement(src).AddFlowElement(sh).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	headers := data.(*Data).ResponseHeaders()

	// Both browser and hardware properties target the Accept-CH header; segments
	// are merged and de-duplicated.
	got := headers["Accept-CH"]
	if got == "" {
		t.Fatalf("expected Accept-CH header, got %v", headers)
	}
	for _, want := range []string{"Sec-CH-UA", "Sec-CH-UA-Mobile", "Sec-CH-UA-Model"} {
		if !contains(got, want) {
			t.Fatalf("Accept-CH %q missing %q", got, want)
		}
	}
	// "Sec-CH-UA-Mobile" appears in both source properties but only once in output.
	if count(got, "Sec-CH-UA-Mobile") != 1 {
		t.Fatalf("expected de-duplicated segments, got %q", got)
	}
}

func TestExtractHeaderName(t *testing.T) {
	// The Identifier has no upper-case characters after its first letter, so the
	// header name starts at the next upper-case letter.
	cases := map[string]string{
		"SetHeaderBrowserAccept-CH":  "Accept-CH",
		"SetHeaderHardwareAccept-CH": "Accept-CH",
		"SetHeaderBrowserUA":         "UA",
		"SetHeaderTest1Header":       "Header", // digit in identifier: scan to next upper-case
	}
	for in, want := range cases {
		if got := extractHeaderName(in); got != want {
			t.Fatalf("extractHeaderName(%q) = %q, want %q", in, got, want)
		}
	}
}

func contains(haystack, needle string) bool {
	for _, seg := range split(haystack) {
		if seg == needle {
			return true
		}
	}
	return false
}

func count(haystack, needle string) int {
	n := 0
	for _, seg := range split(haystack) {
		if seg == needle {
			n++
		}
	}
	return n
}

func split(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			out = append(out, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	out = append(out, cur)
	return out
}
