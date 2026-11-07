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

// fetch sends one request with a fixed User-Agent, as one browser does, and
// with If-None-Match when etag is not empty.
func fetch(t *testing.T, srv *httptest.Server, method, path,
	etag string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("User-Agent", "agent-1")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp
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
	for _, ep := range clientSideEndpoints {
		t.Run(ep.path, func(t *testing.T) {
			srv, engine := newFailingServer(t)

			good := fetch(t, srv, ep.method, ep.path, "")
			etag := good.Header.Get("ETag")
			if good.StatusCode != http.StatusOK || etag == "" {
				t.Fatalf("good request status = %d, ETag = %q",
					good.StatusCode, etag)
			}

			engine.fail.Store(true)
			during := fetch(t, srv, ep.method, ep.path, etag)
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

// etagChildEnv makes TestETagInChildProcess print the ETag of fixedEvidence
// rather than skip.
const etagChildEnv = "FIFTYONE_WEB_ETAG_CHILD"

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
