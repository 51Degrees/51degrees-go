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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
)

// The server route of the browser tests. A server runs the web integration
// over a Cloud Request Engine, serves the page the script the JavaScript
// builder writes, and takes the browser's answers back on the JSON endpoint.
// Each iteration must reach the cloud with the session id, the sequence and
// the browser's answers, and a server must be able to give the visitor's IP
// address, JA4 and HTTP/2 settings as query parameters.
//
// The browser test answers arrive under a 51D_ name that changes with each
// script, so the service can't list it among its Evidence keys. The stand-in
// cloud answers discovery with the list the service gives today, which names
// none of them.

const (
	passThroughEvidenceKeys = `["header.user-agent","query.user-agent",` +
		`"server.client-ip","query.client-ip","query.ja4","query.h2",` +
		`"query.humanconfidence","query.transaction"]`
	answerName = "51D_q7Hx2b"
	answers    = "3z|eJzLSM3JyQcABiwCFQ"
	visitorIP  = "203.0.113.7"
	visitorJA4 = "t13d1516h2_8daaf6152771_02713d6af862"
	visitorH2  = "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p"
)

// recordingCloud is a stand-in cloud service that keeps the form of every
// data request.
type recordingCloud struct {
	mu    sync.Mutex
	forms []url.Values
}

func (c *recordingCloud) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"Products":{}}`)
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, passThroughEvidenceKeys)
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("the cloud could not read the form: %v", err)
		}
		c.mu.Lock()
		c.forms = append(c.forms, r.PostForm)
		c.mu.Unlock()
		io.WriteString(w, `{"device":{}}`)
	})
	return mux
}

func (c *recordingCloud) sent() []url.Values {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]url.Values(nil), c.forms...)
}

// passThroughServer serves the web integration over a Pipeline holding a
// Cloud Request Engine with a License Key, as a publisher's server does.
func passThroughServer(t *testing.T, cloud *recordingCloud) *httptest.Server {
	t.Helper()
	cloudSrv := httptest.NewServer(cloud.handler(t))
	t.Cleanup(cloudSrv.Close)
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	cre, err := requestengine.NewBuilder().
		SetResourceKey("test-resource-key").
		SetLicenseKey("test-license-key").
		SetEndpoint(cloudSrv.URL + "/").
		SetHTTPClient(cloudSrv.Client()).
		SetLogger(quiet).
		Build()
	if err != nil {
		t.Fatalf("build the Cloud Request Engine: %v", err)
	}
	p, err := AssemblePipeline(PipelineConfig{
		Logger:                    quiet,
		SuppressProcessExceptions: true,
	}, cre)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	srv := httptest.NewServer(NewMiddleware(p).Handler(http.NotFoundHandler()))
	t.Cleanup(srv.Close)
	return srv
}

func send(t *testing.T, srv *httptest.Server, req *http.Request) string {
	t.Helper()
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

func supplied() url.Values {
	return url.Values{
		"client-ip": {visitorIP},
		"ja4":       {visitorJA4},
		"h2":        {visitorH2},
	}
}

// expect checks one field of a form the cloud was sent. Evidence names
// ignore letter case, and the engine sends each in lowercase, which the
// service reads as it reads any other spelling.
func expect(t *testing.T, form url.Values, name, want string) {
	t.Helper()
	got := ""
	for key, values := range form {
		if strings.EqualFold(key, name) && len(values) > 0 {
			got = values[0]
		}
	}
	if got != want {
		t.Errorf("the cloud was sent %s=%q, want %q (form %v)", name, got, want, form)
	}
}

func TestEveryIterationReachesTheCloudWithTheSessionSequenceAndAnswers(t *testing.T) {
	cloud := &recordingCloud{}
	srv := passThroughServer(t, cloud)

	// The page's script. The server supplies the visitor's address, JA4 and
	// HTTP/2 settings as query parameters.
	req, _ := http.NewRequest(http.MethodGet,
		srv.URL+DefaultJavaScriptURL+"?"+supplied().Encode(), nil)
	script := send(t, srv, req)
	sent := cloud.sent()
	if len(sent) != 1 {
		t.Fatalf("the page made %d cloud requests, want 1", len(sent))
	}
	first := sent[0]
	sessionID := first.Get("session-id")
	if sessionID == "" {
		t.Fatalf("the first iteration sent no session id: %v", first)
	}
	expect(t, first, "sequence", "1")
	expect(t, first, "client-ip", visitorIP)
	expect(t, first, "ja4", visitorJA4)
	expect(t, first, "h2", visitorH2)
	expect(t, first, "license", "test-license-key")
	if !strings.Contains(script, sessionID) {
		t.Error("the script does not carry the session id")
	}

	// The browser's answers come back on the JSON endpoint, in the form body
	// the script posts and in the cookie the browser tests write, with the
	// session id and sequence the script was given.
	form := url.Values{
		answerName:   {answers},
		"session-id": {sessionID},
		"sequence":   {"1"},
	}
	req, _ = http.NewRequest(http.MethodPost,
		srv.URL+DefaultJSONURL+"?"+supplied().Encode(),
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "51D_BrowserTests", Value: url.QueryEscape(answers)})
	send(t, srv, req)
	sent = cloud.sent()
	if len(sent) != 2 {
		t.Fatalf("the callback made %d cloud requests in all, want 2", len(sent))
	}
	second := sent[1]
	expect(t, second, "session-id", sessionID)
	expect(t, second, "sequence", "2")
	expect(t, second, answerName, answers)
	expect(t, second, "51D_BrowserTests", url.QueryEscape(answers))
	expect(t, second, "client-ip", visitorIP)
	expect(t, second, "ja4", visitorJA4)
	expect(t, second, "h2", visitorH2)
}
