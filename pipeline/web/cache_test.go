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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// The Cache-Control values the client-side endpoints send.
const (
	cacheableControl = "private, max-age=1800"
	failedControl    = "private, no-cache"
)

// failingEngine is an engine whose processing fails while fail is set, as a
// cloud engine's does when the cloud service cannot be reached.
type failingEngine struct {
	*core.BaseElement
	fail atomic.Bool
}

func newFailingEngine() *failingEngine {
	e := &failingEngine{BaseElement: core.NewBaseElement("device",
		core.NewWhitelistFilter("header.user-agent"), nil)}
	e.SetProperties([]core.PropertyMetadata{
		{Name: "ismobile", Element: e, Available: true},
	})
	return e
}

func (e *failingEngine) Process(fd core.FlowData) error {
	if e.fail.Load() {
		return errors.New("the engine could not process the request")
	}
	d := fd.GetOrAdd("device", func(core.FlowData) core.ElementData {
		return core.NewBaseElementData()
	})
	d.Set("ismobile", "false")
	return nil
}

// newFailingServer serves a web pipeline holding a failingEngine. Process
// exceptions are suppressed, as on a live site, so a failure is only
// recorded in the Flow Data.
func newFailingServer(t *testing.T) (*httptest.Server, *failingEngine) {
	t.Helper()
	engine := newFailingEngine()
	p, err := AssemblePipeline(
		PipelineConfig{SuppressProcessExceptions: true}, engine)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	srv := httptest.NewServer(NewMiddleware(p).Handler(http.NotFoundHandler()))
	t.Cleanup(func() {
		srv.Close()
		p.Close()
	})
	return srv, engine
}

// clientSideEndpoints are the two intercepted endpoints, each with the
// method the browser uses for it.
var clientSideEndpoints = []struct{ method, path string }{
	{http.MethodGet, DefaultJavaScriptURL},
	{http.MethodPost, DefaultJSONURL},
}

// clientSidePaths are the two intercepted endpoints, for the tests that
// revalidate with GET.
var clientSidePaths = []string{DefaultJavaScriptURL, DefaultJSONURL}

// testOrigin is the Origin of a page on another site that calls the
// endpoints.
const testOrigin = "https://example.com"

// fetch sends one request with a fixed User-Agent, as one browser does, and
// with If-None-Match when etag is not empty.
func fetch(t *testing.T, srv *httptest.Server, method, path,
	etag string) *http.Response {
	t.Helper()
	resp, _ := fetchFrom(t, srv, method, path, etag, "")
	return resp
}

// fetchFrom is fetch with an Origin header when origin is not empty. It
// returns the body too.
func fetchFrom(t *testing.T, srv *httptest.Server, method, path, etag,
	origin string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("User-Agent", "agent-1")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, string(body)
}

// A response to a request whose processing recorded errors carries no ETag,
// so the browser that kept it has nothing to revalidate with and gets a full
// response once processing works again, rather than a 304 that keeps the
// faulty one.
func TestFailedResponseIsNotKeptOnceProcessingRecovers(t *testing.T) {
	for _, ep := range clientSideEndpoints {
		t.Run(ep.path, func(t *testing.T) {
			srv, engine := newFailingServer(t)

			engine.fail.Store(true)
			failed := fetch(t, srv, ep.method, ep.path, "")
			if failed.StatusCode != http.StatusOK {
				t.Fatalf("failed request status = %d, want 200",
					failed.StatusCode)
			}
			if got := failed.Header.Get("Cache-Control"); got != failedControl {
				t.Errorf("failed response Cache-Control = %q, want %q",
					got, failedControl)
			}
			if got := failed.Header.Get("ETag"); got != "" {
				t.Errorf("failed response carries the ETag %s, want none", got)
			}

			// The browser revalidates with whatever ETag it was given.
			engine.fail.Store(false)
			recovered := fetch(t, srv, ep.method, ep.path,
				failed.Header.Get("ETag"))
			if recovered.StatusCode != http.StatusOK {
				t.Fatalf("status once processing works = %d, want 200, so "+
					"the browser was told to keep the failed response",
					recovered.StatusCode)
			}
			if got := recovered.Header.Get("Cache-Control"); got != cacheableControl {
				t.Errorf("recovered response Cache-Control = %q, want %q",
					got, cacheableControl)
			}
			if recovered.Header.Get("ETag") == "" {
				t.Error("recovered response carries no ETag")
			}
		})
	}
}

// A browser that holds a good response keeps it while processing fails,
// because a matching If-None-Match still gets its 304. The 304 carries the
// Cache-Control and ETag a 200 to the same request would carry (RFC 9110
// section 15.4.5), so it has no ETag and the browser asks again next time.
func TestGoodResponseIsKeptWhileProcessingFails(t *testing.T) {
	for _, path := range clientSidePaths {
		t.Run(path, func(t *testing.T) {
			srv, engine := newFailingServer(t)

			good := fetch(t, srv, http.MethodGet, path, "")
			etag := good.Header.Get("ETag")
			if good.StatusCode != http.StatusOK || etag == "" {
				t.Fatalf("good request status = %d, ETag = %q",
					good.StatusCode, etag)
			}

			engine.fail.Store(true)
			during := fetch(t, srv, http.MethodGet, path, etag)
			if during.StatusCode != http.StatusNotModified {
				t.Fatalf("status while processing fails = %d, want 304",
					during.StatusCode)
			}
			if got := during.Header.Get("Cache-Control"); got != failedControl {
				t.Errorf("304 Cache-Control = %q, want %q", got, failedControl)
			}
			if got := during.Header.Get("ETag"); got != "" {
				t.Errorf("304 carries the ETag %s, want none", got)
			}
		})
	}
}

// A GET or HEAD whose If-None-Match matches gets a 304 with no content,
// carrying the Cache-Control, Vary and ETag headers the 200 carried (RFC 9110
// section 15.4.5) and the CORS header, so a page on another site can read it.
// It carries no Content-Type, as a 304 sends no representation metadata other
// than those.
func TestNotModifiedCarriesTheHeadersOfThe200(t *testing.T) {
	for _, path := range clientSidePaths {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(method+path, func(t *testing.T) {
				srv, _ := newFailingServer(t)

				good, _ := fetchFrom(t, srv, http.MethodGet, path, "",
					testOrigin)
				etag := good.Header.Get("ETag")
				if good.StatusCode != http.StatusOK || etag == "" {
					t.Fatalf("good request status = %d, ETag = %q",
						good.StatusCode, etag)
				}

				resp, body := fetchFrom(t, srv, method, path, etag, testOrigin)
				if resp.StatusCode != http.StatusNotModified {
					t.Fatalf("status = %d, want 304", resp.StatusCode)
				}
				if body != "" {
					t.Errorf("304 has the content %q, want none", body)
				}
				for _, name := range []string{"Cache-Control", "Vary", "ETag",
					"Access-Control-Allow-Origin"} {
					got, want := resp.Header.Get(name), good.Header.Get(name)
					if got != want || got == "" {
						t.Errorf("304 %s = %q, want the 200's %q",
							name, got, want)
					}
				}
				if got := resp.Header.Get("Content-Type"); got != "" {
					t.Errorf("304 Content-Type = %q, want none", got)
				}
			})
		}
	}
}

// Any method other than GET or HEAD whose If-None-Match matches gets 412
// Precondition Failed, never a 304 (RFC 9110 sections 13.1.2 and 9.3.3),
// whether processing works or fails. The 412 has no content and carries only
// the CORS header, so a page on another site can read the status.
func TestMatchingETagOnPostGetsPreconditionFailed(t *testing.T) {
	for _, path := range clientSidePaths {
		for _, fails := range []bool{false, true} {
			name := fmt.Sprintf("%s/fails=%v", path, fails)
			t.Run(name, func(t *testing.T) {
				srv, engine := newFailingServer(t)

				good := fetch(t, srv, http.MethodPost, path, "")
				etag := good.Header.Get("ETag")
				if good.StatusCode != http.StatusOK || etag == "" {
					t.Fatalf("good request status = %d, ETag = %q",
						good.StatusCode, etag)
				}

				engine.fail.Store(fails)
				resp, body := fetchFrom(t, srv, http.MethodPost, path, etag,
					testOrigin)
				if resp.StatusCode != http.StatusPreconditionFailed {
					t.Fatalf("status = %d, want 412", resp.StatusCode)
				}
				if body != "" {
					t.Errorf("412 has the content %q, want none", body)
				}
				got := resp.Header.Get("Access-Control-Allow-Origin")
				if got != testOrigin {
					t.Errorf("412 Access-Control-Allow-Origin = %q, want %q",
						got, testOrigin)
				}
				for _, name := range []string{"Cache-Control", "Vary", "ETag",
					"Content-Type"} {
					if got := resp.Header.Get(name); got != "" {
						t.Errorf("412 %s = %q, want none", name, got)
					}
				}
			})
		}
	}
}

// etagChildEnv makes TestETagInChildProcess print the ETag of fixedEvidence
// rather than skip.
const etagChildEnv = "_51DEGREES_WEB_ETAG_CHILD"

// fixedEvidence holds a value for every key the web pipeline's filter takes
// that a browser's request carries.
var fixedEvidence = map[string]any{
	"header.user-agent":        "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
	"header.host":              "example.com",
	"header.protocol":          "https",
	"query.fod-js-object-name": "fod",
}

func fixedEvidenceETag(t *testing.T) string {
	t.Helper()
	p := buildWebPipeline(t)
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidenceMap(fixedEvidence)
	return NewMiddleware(p).etag(fd)
}

// TestETagInChildProcess prints the ETag of fixedEvidence when
// TestSameEvidenceGivesSameETagInEveryProcess starts it in a new process.
func TestETagInChildProcess(t *testing.T) {
	if os.Getenv(etagChildEnv) != "1" {
		t.Skip("runs only in the process the ETag test starts")
	}
	fmt.Printf("etag=%s\n", fixedEvidenceETag(t))
}

// The same Evidence gives the same ETag in a new process, as it must for a
// browser revalidating after a restart or against another server.
func TestSameEvidenceGivesSameETagInEveryProcess(t *testing.T) {
	want := fixedEvidenceETag(t)

	cmd := exec.Command(os.Args[0],
		"-test.run=^TestETagInChildProcess$", "-test.count=1")
	cmd.Env = append(os.Environ(), etagChildEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child process: %v\n%s", err, out)
	}
	got := ""
	for _, line := range strings.Split(string(out), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "etag="); ok {
			got = v
		}
	}
	if got == "" {
		t.Fatalf("child process printed no ETag:\n%s", out)
	}
	if got != want {
		t.Fatalf("ETag in a new process = %s, want %s", got, want)
	}
	t.Logf("ETag %s in both processes", want)
}

// The ETag hashes the Evidence alone, so a data file update can change the
// content while the ETag stays the same. It is therefore a weak entity tag,
// W/ then a quoted string (RFC 9110 sections 8.8.1 and 8.8.3).
func TestETagIsWeak(t *testing.T) {
	for _, ep := range clientSideEndpoints {
		t.Run(ep.path, func(t *testing.T) {
			srv, _ := newFailingServer(t)
			etag := fetch(t, srv, ep.method, ep.path, "").Header.Get("ETag")
			opaque, weak := strings.CutPrefix(etag, "W/")
			if !weak || len(opaque) < 3 || !strings.HasPrefix(opaque, `"`) ||
				!strings.HasSuffix(opaque, `"`) {
				t.Fatalf("ETag = %s, want W/ then a quoted string", etag)
			}
		})
	}
}

// If-None-Match is compared with the weak comparison function (RFC 9110
// sections 13.1.2 and 8.8.3.2), so the quoted parts are compared whether
// or not either tag carries W/. The field may list several tags, over one
// or more header lines, or be "*". A field that is neither "*" nor a comma
// separated list of entity tags does not match (section 13.1.2, step 3). A
// match gets 304 to a GET and 412 to a POST, as for an exact match.
func TestIfNoneMatchUsesWeakComparison(t *testing.T) {
	srv, _ := newFailingServer(t)
	etag := fetch(t, srv, http.MethodGet, DefaultJSONURL, "").Header.Get("ETag")
	opaque := strings.TrimPrefix(etag, "W/")
	if opaque == "" {
		t.Fatal("the response carries no ETag")
	}
	cases := []struct {
		name  string
		lines []string
		match bool
	}{
		{"weak", []string{"W/" + opaque}, true},
		{"strong", []string{opaque}, true},
		{"list", []string{`"other", W/` + opaque}, true},
		{"list without spaces", []string{`W/"other",` + opaque}, true},
		{"two header lines", []string{`"other"`, "W/" + opaque}, true},
		{"any", []string{"*"}, true},
		{"other tag", []string{`W/"other"`}, false},
		{"other tags", []string{`"other", W/"another"`}, false},
		{"lower case weak prefix", []string{"w/" + opaque}, false},
		{"unquoted", []string{strings.Trim(opaque, `"`)}, false},
		{"tag then a value that is not a tag", []string{"W/" + opaque + ", other"}, false},
		{"two tags with no comma", []string{`"other" W/` + opaque}, false},
		{"a value that is not a tag, then the tag", []string{"other, W/" + opaque}, false},
		{"space inside the quotes", []string{`"oth er", W/` + opaque}, false},
		{"star in a list", []string{`"other", *`}, false},
		{"list with a tab", []string{"\"other\",\tW/" + opaque}, true},
	}
	for _, tc := range cases {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				req, err := http.NewRequest(method, srv.URL+DefaultJSONURL, nil)
				if err != nil {
					t.Fatalf("request: %v", err)
				}
				req.Header.Set("User-Agent", "agent-1")
				for _, line := range tc.lines {
					req.Header.Add("If-None-Match", line)
				}
				resp, err := srv.Client().Do(req)
				if err != nil {
					t.Fatalf("%s: %v", method, err)
				}
				_ = resp.Body.Close()
				want := http.StatusOK
				if tc.match && method == http.MethodGet {
					want = http.StatusNotModified
				} else if tc.match {
					want = http.StatusPreconditionFailed
				}
				if resp.StatusCode != want {
					t.Errorf("If-None-Match %q gives %d, want %d",
						tc.lines, resp.StatusCode, want)
				}
			})
		}
	}
}
