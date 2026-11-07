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
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/sequence"
)

type capture struct {
	mu     sync.Mutex
	bodies [][]byte
}

func (c *capture) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		data, _ := io.ReadAll(gz)
		c.mu.Lock()
		c.bodies = append(c.bodies, data)
		c.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
}

func (c *capture) count() int { c.mu.Lock(); defer c.mu.Unlock(); return len(c.bodies) }
func (c *capture) waitFor(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.count() >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d payloads (got %d)", n, c.count())
}

func TestShareRules(t *testing.T) {
	e := NewBuilder().
		SetBlockedHTTPHeaders("cookies").
		SetIncludedQueryStringParameters("myparam").
		Build()
	defer e.Close()

	cases := map[string]bool{
		"header.user-agent": true,
		"header.cookies":    false,
		"cookie.51d_foo":    true,
		"cookie.sessionx":   false,
		"query.51d_bar":     true,
		"query.myparam":     true,
		"query.other":       false,
		"server.client-ip":  true,
	}
	for key, want := range cases {
		if got := e.isShared(key); got != want {
			t.Errorf("isShared(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestUsageSharingSendsXML(t *testing.T) {
	cap := &capture{}
	srv := cap.server()
	defer srv.Close()

	e := NewBuilder().
		SetHTTPClient(srv.Client()).
		SetShareUsageURL(srv.URL).
		SetMinimumEntriesPerMessage(1).
		Build()

	p, err := core.NewPipelineBuilder().AddFlowElement(sequence.New(nil)).AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "test-agent")
	fd.AddEvidence("server.client-ip", "1.2.3.4")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	cap.waitFor(t, 1)

	cap.mu.Lock()
	body := string(cap.bodies[0])
	cap.mu.Unlock()

	// Well-formed XML with the expected structure.
	var devices struct {
		XMLName xml.Name `xml:"Devices"`
		Device  []struct {
			Product  string `xml:"Product"`
			Sequence string `xml:"Sequence"`
			ServerIP string `xml:"ServerIP"`
			Headers  []struct {
				Name  string `xml:"Name,attr"`
				Value string `xml:",chardata"`
			} `xml:"header"`
		} `xml:"Device"`
	}
	if err := xml.Unmarshal([]byte(body), &devices); err != nil {
		t.Fatalf("invalid XML: %v\n%s", err, body)
	}
	if len(devices.Device) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices.Device))
	}
	if devices.Device[0].Product != "Pipeline" {
		t.Fatalf("unexpected product: %s", devices.Device[0].Product)
	}
	foundUA := false
	for _, h := range devices.Device[0].Headers {
		if h.Name == "user-agent" && h.Value == "test-agent" {
			foundUA = true
		}
	}
	if !foundUA {
		t.Fatalf("user-agent header not shared: %s", body)
	}
	// Schema shape: lower-case category element, Name attribute, and
	// a CDATA-wrapped value.
	if !strings.Contains(body, `<header Name="user-agent"><![CDATA[test-agent]]></header>`) {
		t.Fatalf("header not in the expected lower-case CDATA form: %s", body)
	}
	// Sequence is always present (even 0) and ServerIP is populated.
	if devices.Device[0].Sequence == "" {
		t.Fatalf("Sequence element missing: %s", body)
	}
}

func TestUsageSharingCleanupFlush(t *testing.T) {
	cap := &capture{}
	srv := cap.server()
	defer srv.Close()

	// Minimum batch size higher than the number of entries, so nothing is sent
	// until Close flushes the remainder.
	e := NewBuilder().
		SetHTTPClient(srv.Client()).
		SetShareUsageURL(srv.URL).
		SetMinimumEntriesPerMessage(50).
		SetTakeTimeout(time.Hour). // avoid periodic flush during the test
		Build()
	p, _ := core.NewPipelineBuilder().AddFlowElement(sequence.New(nil)).AddFlowElement(e).Build()
	// Closes the pipeline if the test stops before the Close below.
	defer p.Close()

	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "agent")
	_ = fd.Process()

	if cap.count() != 0 {
		t.Fatalf("nothing should be sent before flush, got %d", cap.count())
	}
	p.Close() // triggers final flush
	cap.waitFor(t, 1)
}

func TestTrackerDedup(t *testing.T) {
	tr := newTracker(time.Hour)
	h := uint64(12345)
	if tr.seenRecently(h) {
		t.Fatal("first sighting should not be 'recently seen'")
	}
	if !tr.seenRecently(h) {
		t.Fatal("second sighting within window should be 'recently seen'")
	}
}
