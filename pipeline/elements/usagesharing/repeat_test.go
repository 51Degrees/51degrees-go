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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/sequence"
)

// Only the query values session-id and sequence are left out of the repeat
// comparison. A header, cookie or other value with either name is compared
// like any other value, so a call where it changed is not a repeat.
func TestRepeatHashLeavesOutOnlyTheQuerySessionIDAndSequence(t *testing.T) {
	with := func(key, value string) map[string]string {
		return map[string]string{"header.user-agent": "agent", key: value}
	}
	for _, key := range []string{"query.session-id", "query.sequence", "query.Sequence"} {
		if hashEvidence(with(key, "1")) != hashEvidence(with(key, "2")) {
			t.Errorf("%s is compared, so every call in a session is new", key)
		}
	}
	for _, key := range []string{
		"header.sequence", "header.session-id",
		"cookie.session-id", "server.sequence",
	} {
		if hashEvidence(with(key, "1")) == hashEvidence(with(key, "2")) {
			t.Errorf("%s is not compared, so a call where only it "+
				"changed is dropped as a repeat", key)
		}
	}
}

// sequencePipeline builds a Pipeline with a Sequence Element and a usage
// sharing element that sends each entry as it comes to a capture.
func sequencePipeline(t *testing.T) (core.Pipeline, *capture) {
	t.Helper()
	c := &capture{}
	srv := c.server()
	t.Cleanup(srv.Close)
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	e, err := Factory(map[string]any{"ShareUsageUrl": srv.URL, "MinimumEntriesPerMessage": 1}, quiet)
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	p, err := core.NewPipelineBuilder().SetLogger(quiet).
		AddFlowElement(sequence.New(quiet)).AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p, c
}

var (
	sequenceTag  = regexp.MustCompile(`<Sequence>(\d+)</Sequence>`)
	sessionIDTag = regexp.MustCompile(`<SessionId>([^<]*)</SessionId>`)
)

// devices returns every record the capture holds.
func devices(c *capture) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, body := range c.bodies {
		for _, part := range strings.Split(string(body), "<Device>")[1:] {
			out = append(out, part)
		}
	}
	return out
}

// One session of three calls, as the JavaScript makes them. The second has
// the same values as the first and only the next sequence, so it is a repeat
// and not shared. The third changes a 51D_ value, so it is shared, with the
// same session id and its own sequence.
func TestASessionSharesTheCallsThatChanged(t *testing.T) {
	p, c := sequencePipeline(t)
	session := ""
	call := func(sequenceIn, width string) {
		t.Helper()
		fd := p.CreateFlowData()
		fd.AddEvidence("header.user-agent", "ua")
		fd.AddEvidence("query.51D_ScreenPixelsWidth", width)
		if session != "" {
			fd.AddEvidence(sequence.EvidenceSessionID, session)
		}
		if sequenceIn != "" {
			fd.AddEvidence(sequence.EvidenceSequence, sequenceIn)
		}
		if err := fd.Process(); err != nil {
			t.Fatalf("process: %v", err)
		}
		data, err := fd.Get(sequence.DataKey)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := data.Get(sequence.SessionIDProperty)
		session, _ = id.(string)
	}
	call("", "1080")
	call("1", "1080")
	call("2", "1440")
	waitForEntries(t, c, 2)
	settle(t, c, 2)

	records := devices(c)
	sort.Slice(records, func(i, j int) bool {
		return sequenceTag.FindString(records[i]) < sequenceTag.FindString(records[j])
	})
	var sequences []string
	for _, record := range records {
		if m := sequenceTag.FindStringSubmatch(record); m != nil {
			sequences = append(sequences, m[1])
		}
		if m := sessionIDTag.FindStringSubmatch(record); m == nil || m[1] != session {
			t.Errorf("a record has the session id %v, want %s", m, session)
		}
	}
	if strings.Join(sequences, ",") != "1,3" {
		t.Errorf("the records have the sequences %v, want 1 and 3", sequences)
	}
	if len(records) == 2 && !strings.Contains(strings.ToLower(records[1]), `name="51d_screenpixelswidth">1440<`) {
		t.Errorf("the third call's record lacks the new value:\n%s", records[1])
	}
}

// A request's JA4 and HTTP/2 fingerprints and its billing key reach the
// usage sharing message, as every value given to the Pipeline does.
func TestTheAdditionalValuesAreShared(t *testing.T) {
	p, c := sessionPipeline(t, nil)
	fd := p.CreateFlowData()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("User-Agent", "ua")
	fd.AddEvidence(core.EvidenceHTTPRequest, core.NewWebRequest(r))
	values := map[string]string{
		"server.ja4":            "t13d1516h2_8daaf6152771_02713d6af862",
		"server.h2":             "1:65536;2:0;4:6291456|15663105|0|m,a,s,p",
		"fiftyone.resource-key": "test-resource-key",
	}
	for key, value := range values {
		fd.AddEvidence(key, value)
	}
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	waitForEntries(t, c, 1)
	record := strings.ToLower(strings.Join(devices(c), ""))
	for key, value := range values {
		_, field := core.SplitEvidenceKey(key)
		if want := `name="` + field + `">` + strings.ToLower(value) + `<`; !strings.Contains(record, want) {
			t.Errorf("%s is not in the message:\n%s", key, record)
		}
	}
}

var platformTag = regexp.MustCompile(`<Platform>([^<]*)</Platform>`)

// Platform names the operating system and its version.
// See https://github.com/51Degrees/specifications/blob/6570727d2af6967390d3c4fbec77b9cf0540d3f3/pipeline-specification/pipeline-elements/usage-sharing-element.md?plain=1#L202
func TestPlatformNamesTheVersion(t *testing.T) {
	p, c := sessionPipeline(t, nil)
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	waitForEntries(t, c, 1)
	m := platformTag.FindStringSubmatch(strings.Join(devices(c), ""))
	if m == nil {
		t.Fatal("the record has no Platform")
	}
	if !strings.HasPrefix(m[1], runtime.GOOS+" ") ||
		!strings.ContainsAny(strings.TrimPrefix(m[1], runtime.GOOS), "0123456789") {
		t.Errorf("Platform is %q, want %s and its version", m[1], runtime.GOOS)
	}
}
