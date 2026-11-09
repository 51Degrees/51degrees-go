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

// These tests follow the track session and session cookie name options.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/usage-sharing-element.md#session-tracking

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// sessionPipeline builds a Pipeline holding an element made by Factory from
// params, sending each entry as it comes to a capture of what is shared.
func sessionPipeline(t *testing.T, params map[string]any) (core.Pipeline, *capture) {
	t.Helper()
	c := &capture{}
	srv := c.server()
	t.Cleanup(srv.Close)
	all := map[string]any{
		"ShareUsageUrl":            srv.URL,
		"MinimumEntriesPerMessage": 1,
	}
	for k, v := range params {
		all[k] = v
	}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	e, err := Factory(all, quiet)
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	p, err := core.NewPipelineBuilder().SetLogger(quiet).AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p, c
}

// shareSession processes the same Evidence each time, with the session
// cookie given as Evidence and in the web request, either left out when "".
func shareSession(t *testing.T, p core.Pipeline, evidence, request string) {
	t.Helper()
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	if evidence != "" {
		fd.AddEvidence("cookie.sessionid", evidence)
	}
	if request != "" {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(&http.Cookie{Name: "sessionid", Value: request})
		fd.AddEvidence(core.EvidenceHTTPRequest, core.NewWebRequest(r))
	}
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
}

// settle waits long enough for any further message to arrive, and checks
// that the capture holds want messages.
func settle(t *testing.T, c *capture, want int) {
	t.Helper()
	time.Sleep(400 * time.Millisecond)
	if got := c.count(); got != want {
		t.Errorf("shared %d messages, want %d", got, want)
	}
}

// With track session on, identical Evidence from a new session is shared,
// while a repeat within the same session is not. The session cookie itself
// is not shared.
func TestTrackSessionSharesRepeatFromNewSession(t *testing.T) {
	p, c := sessionPipeline(t, map[string]any{
		"TrackSession": true, "SessionCookieName": "sessionid"})
	shareSession(t, p, "A", "")
	shareSession(t, p, "A", "")
	shareSession(t, p, "B", "")
	c.waitFor(t, 2)
	settle(t, c, 2)
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, body := range c.bodies {
		if strings.Contains(strings.ToLower(string(body)), "sessionid") {
			t.Errorf("the session cookie was shared in %s", body)
		}
	}
}

// The session cookie is read from the Evidence and from the web request,
// and when both hold it the Evidence value counts.
func TestSessionCookieFromEvidenceAndWebRequest(t *testing.T) {
	p, c := sessionPipeline(t, map[string]any{
		"TrackSession": true, "SessionCookieName": "SessionId"})
	shareSession(t, p, "C", "A") // session C, from the Evidence
	shareSession(t, p, "", "C")  // session C again, from the web request
	shareSession(t, p, "", "D")  // a new session, from the web request
	c.waitFor(t, 2)
	settle(t, c, 2)
}

// With track session off, the session cookie is not compared, so identical
// Evidence from another session is a repeat.
func TestTrackSessionOffComparesNoCookie(t *testing.T) {
	p, c := sessionPipeline(t, map[string]any{"SessionCookieName": "sessionid"})
	shareSession(t, p, "A", "")
	shareSession(t, p, "B", "")
	c.waitFor(t, 1)
	settle(t, c, 1)
}

// The session cookie is not shared unless all Evidence is shared, even when
// its name starts with 51D_ as the cookies that are shared do.
func TestSessionCookieSharedOnlyWithAllEvidence(t *testing.T) {
	for _, shareAll := range []bool{false, true} {
		p, c := sessionPipeline(t, map[string]any{
			"TrackSession":      true,
			"SessionCookieName": "51D_session",
			"ShareAllEvidence":  shareAll,
		})
		fd := p.CreateFlowData()
		fd.AddEvidence("header.user-agent", "ua")
		fd.AddEvidence("cookie.51D_session", "A")
		if err := fd.Process(); err != nil {
			t.Fatalf("process: %v", err)
		}
		c.waitFor(t, 1)
		c.mu.Lock()
		body := strings.ToLower(string(c.bodies[0]))
		c.mu.Unlock()
		if shared := strings.Contains(body, "51d_session"); shared != shareAll {
			t.Errorf("with all Evidence shared %v, the session cookie shared "+
				"is %v, in %s", shareAll, shared, body)
		}
	}
}

// Track session with no session cookie name has nothing to compare, so the
// configuration is refused.
func TestTrackSessionNeedsCookieName(t *testing.T) {
	_, err := Factory(map[string]any{"TrackSession": true}, nil)
	var config *core.ConfigurationError
	if !errors.As(err, &config) {
		t.Errorf("Factory gives %v, want a configuration error", err)
	}
}
