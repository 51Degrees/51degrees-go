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

package web

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

// A request that carries a sealed Human Confidence result redeems it once,
// so a client-side endpoint never answers it from what the browser kept,
// with a 304, and tells the browser not to keep its answer.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/caching.md
func TestASealedResultIsNeverAnsweredFromACache(t *testing.T) {
	srv, engine := newFailingServer(t)
	plain, err := http.Get(srv.URL + "/51dpipeline/json")
	if err != nil {
		t.Fatal(err)
	}
	plain.Body.Close()
	etag := plain.Header.Get("ETag")
	if etag == "" {
		t.Fatal("a request with no sealed result got no ETag")
	}
	for _, path := range []string{"/51dpipeline/json", "/51Degrees.core.js"} {
		before := engine.calls.Load()
		request, _ := http.NewRequest(http.MethodGet,
			srv.URL+path+"?humanconfidence=made-up-sealed-value", nil)
		request.Header.Set("If-None-Match", etag)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK || engine.calls.Load() == before {
			t.Errorf("%s: a sealed request got %d without running the Pipeline", path, response.StatusCode)
		}
		if got := response.Header.Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control is %q, want no-store", path, got)
		}
		if response.Header.Get("ETag") != "" {
			t.Errorf("%s: a sealed request's answer has an ETag", path)
		}
	}
}

// A query that can't be parsed is logged without the characters of the bad
// escape, which can be part of a sealed result the page sent.
func TestAQueryThatCantBeParsedIsLoggedWithoutItsText(t *testing.T) {
	var log bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	defer slog.SetDefault(previous)
	srv, _ := newFailingServer(t)
	response, err := http.Get(srv.URL + "/51dpipeline/json?humanconfidence=made-up-sealed%zzvalue")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if !strings.Contains(log.String(), "could not parse request query") {
		t.Fatalf("the parse failure was not logged: %s", log.String())
	}
	if strings.Contains(log.String(), "%zz") {
		t.Errorf("the log holds part of the query: %s", log.String())
	}
}
