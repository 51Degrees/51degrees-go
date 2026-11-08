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

// Every way of printing a WebRequest gives the placeholder, and the request
// is still available to the code that asks for it.
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
	if w.Request() != r {
		t.Error("Request does not return the request held")
	}
	if (WebRequest{}).Request() != nil {
		t.Error("the zero WebRequest holds a request")
	}
}
