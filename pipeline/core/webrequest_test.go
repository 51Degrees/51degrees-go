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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Every way of printing a WebRequest gives the placeholder.
func TestWebRequestPrintsAPlaceholder(t *testing.T) {
	r := httptest.NewRequest("GET", "/page?token=query-secret", nil)
	r.Header.Set("Authorization", "header-secret")
	r.AddCookie(&http.Cookie{Name: "session", Value: "cookie-secret"})
	w := NewWebRequest(r)

	e := newEvidence()
	e.put(EvidenceHTTPRequest, w)
	for _, v := range []string{"%v", "%+v", "%#v", "%s", "%q", "%d"} {
		for name, got := range map[string]string{
			"value":    fmt.Sprintf(v, w),
			"pointer":  fmt.Sprintf(v, &w),
			"evidence": fmt.Sprintf(v, e.AsMap()),
		} {
			if strings.Contains(got, "secret") {
				t.Errorf("%s with %s printed %.120s", name, v, got)
			}
		}
	}
	if got := w.String(); got != webRequestPlaceholder {
		t.Errorf("String = %q, want %q", got, webRequestPlaceholder)
	}
}

// The view gives the named parts of the request and nothing that changes it.
func TestWebRequestNamedParts(t *testing.T) {
	r := httptest.NewRequest("POST", "http://example.com/a%2Fb/c?x=1&y=2", nil)
	r.Header.Set("X-Test", "t")
	r.Header.Add("Cookie", "a=1; 51D_b=2")
	w := NewWebRequest(r).WithClientIP("203.0.113.5").WithProtocol("https").
		WithSession(map[string]string{"51D_s": "v"})

	if w.Method() != "POST" || w.ClientIP() != "203.0.113.5" ||
		w.Protocol() != "https" {
		t.Errorf("method %q, client IP %q, protocol %q", w.Method(),
			w.ClientIP(), w.Protocol())
	}
	if w.Path() != "/a%2Fb/c" {
		t.Errorf("path = %q, want the request line's /a%%2Fb/c", w.Path())
	}
	if h := w.Headers(); h.Get("X-Test") != "t" || h.Get("Host") != "example.com" {
		t.Errorf("headers = %v", h)
	}
	if c := w.Cookies(); len(c) != 2 || c[1].Name != "51D_b" || c[1].Value != "2" {
		t.Errorf("cookies = %v", c)
	}
	if q := w.Query(); q.Get("x") != "1" || q.Get("y") != "2" {
		t.Errorf("query = %v", q)
	}
	if w.Session()["51D_s"] != "v" {
		t.Errorf("session = %v", w.Session())
	}

	w.Headers().Set("X-Test", "changed")
	w.Query().Set("x", "changed")
	w.Session()["51D_s"] = "changed"
	if w.Headers().Get("X-Test") != "t" || w.Query().Get("x") != "1" ||
		w.Session()["51D_s"] != "v" {
		t.Error("changing a returned part changed the view")
	}

	z := WebRequest{}
	if z.Method() != "" || z.Path() != "" || len(z.Headers()) != 0 ||
		len(z.Cookies()) != 0 || len(z.Query()) != 0 || len(z.Session()) != 0 {
		t.Error("the zero WebRequest has parts")
	}
}
