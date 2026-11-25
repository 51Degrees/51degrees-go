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
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// closingData is Element Data that holds something to clean up, and counts
// how many times it is closed.
type closingData struct {
	*core.BaseElementData
	closed atomic.Int32
}

func (d *closingData) Close() error {
	d.closed.Add(1)
	return nil
}

// closingEngine adds a closingData to each Flow Data it processes and sends
// it on seen.
type closingEngine struct {
	*core.BaseElement
	seen chan *closingData
}

func (e *closingEngine) Process(fd core.FlowData) error {
	data := &closingData{BaseElementData: core.NewBaseElementData()}
	fd.GetOrAdd("device", func(core.FlowData) core.ElementData {
		return data
	}).Set("ismobile", "false")
	e.seen <- data
	return nil
}

// The middleware closes the Flow Data of each request when the request ends,
// and that cleans up the Element Data that needs it. So a page that never
// reads the Element Data, and the JavaScript and JSON endpoints, leave
// nothing to the garbage collector.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md?plain=1#L5-L18
func TestTheMiddlewareCleansUpElementDataThatNeedsIt(t *testing.T) {
	engine := &closingEngine{
		BaseElement: core.NewBaseElement("device",
			core.NewWhitelistFilter("header.user-agent"), nil),
		seen: make(chan *closingData, 1),
	}
	engine.SetProperties([]core.PropertyMetadata{
		{Name: "ismobile", Element: engine, Available: true},
	})
	p, err := AssemblePipeline(PipelineConfig{}, engine)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	defer p.Close()
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "page")
	})
	srv := httptest.NewServer(NewMiddleware(p).Handler(page))
	defer srv.Close()

	requests := append([]struct{ method, path string }{
		{http.MethodGet, "/page"}}, clientSideEndpoints...)
	seen := make([]*closingData, len(requests))
	for i, r := range requests {
		fetch(t, srv, r.method, r.path, "")
		seen[i] = <-engine.seen
	}
	// The middleware closes the Flow Data after the handler returns, which
	// can be after the client has read the whole response. Close waits for
	// every handler to return, so the counts are final after it.
	srv.Close()
	for i, r := range requests {
		if n := seen[i].closed.Load(); n != 1 {
			t.Errorf("after %s %s the Element Data was closed %d times, "+
				"want once", r.method, r.path, n)
		}
	}
}
