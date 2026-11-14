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

package usagesharing

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// sentWaitSeconds bounds the wait for a message. Closing the Pipeline sends
// the last message before it returns, so the wait only guards a hang.
const sentWaitSeconds = 5

// newCollector starts a server that keeps the body of each message the
// element sends, unzipped.
func newCollector(t *testing.T) (*httptest.Server, <-chan string) {
	t.Helper()
	bodies := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			zr, err := gzip.NewReader(r.Body)
			if err == nil {
				b, _ := io.ReadAll(zr)
				bodies <- string(b)
			}
			w.WriteHeader(http.StatusOK)
		}))
	t.Cleanup(srv.Close)
	return srv, bodies
}

// The ClientIP of a message is the client IP address the element finds,
// from the Evidence server.client-ip or else from the web request in the
// Evidence, and the Evidence value counts when both hold one.
func TestClientIPIsTheOneFound(t *testing.T) {
	for _, test := range []struct {
		name     string
		evidence string
		request  string
		want     string
	}{
		{"web request only", "", "203.0.113.7", "203.0.113.7"},
		{"Evidence only", "198.51.100.4", "", "198.51.100.4"},
		{"both", "198.51.100.4", "203.0.113.7", "198.51.100.4"},
	} {
		t.Run(test.name, func(t *testing.T) {
			collector, bodies := newCollector(t)
			e := NewBuilder().SetMinimumEntriesPerMessage(1).
				SetShareUsageURL(collector.URL).Build()
			p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			fd := p.CreateFlowData()
			fd.AddEvidence("header.user-agent", "client-ip-test")
			if test.evidence != "" {
				fd.AddEvidence("server.client-ip", test.evidence)
			}
			if test.request != "" {
				fd.AddEvidence(httpRequestKey, core.NewWebRequest(
					httptest.NewRequest("GET", "/", nil)).
					WithClientIP(test.request))
			}
			if err := fd.Process(); err != nil {
				t.Fatalf("process: %v", err)
			}
			p.Close()

			select {
			case got := <-bodies:
				want := "<ClientIP>" + test.want + "</ClientIP>"
				if !strings.Contains(got, want) {
					t.Errorf("the message lacks %s: %.600s", want, got)
				}
			case <-time.After(sentWaitSeconds * time.Second):
				t.Fatal("nothing was sent")
			}
		})
	}
}
