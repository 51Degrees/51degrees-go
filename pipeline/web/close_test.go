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
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// recordingEngine adds Element Data and sends each Flow Data it processes
// on seen.
type recordingEngine struct {
	*core.BaseElement
	seen chan core.FlowData
}

func (e *recordingEngine) Process(fd core.FlowData) error {
	fd.GetOrAdd("device", func(core.FlowData) core.ElementData {
		return core.NewBaseElementData()
	}).Set("ismobile", "false")
	e.seen <- fd
	return nil
}

// The middleware closes the Flow Data of each request when the request ends,
// so its Element Data goes back to the pools. The page's handler reads it
// while the request runs.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/web-integration.md?plain=1#L27-L28
func TestTheMiddlewareClosesTheFlowDataWhenTheRequestEnds(t *testing.T) {
	engine := &recordingEngine{
		BaseElement: core.NewBaseElement("device",
			core.NewWhitelistFilter("header.user-agent"), nil),
		seen: make(chan core.FlowData, 1),
	}
	engine.SetProperties([]core.PropertyMetadata{
		{Name: "ismobile", Element: engine, Available: true},
	})
	p, err := AssemblePipeline(PipelineConfig{}, engine)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	defer p.Close()
	readDuringRequest := make(chan error, 1)
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fd, _ := FromContext(r.Context())
		_, err := fd.Get("device")
		readDuringRequest <- err
		io.WriteString(w, "page")
	})
	srv := httptest.NewServer(NewMiddleware(p).Handler(page))
	defer srv.Close()

	requests := append([]struct{ method, path string }{
		{http.MethodGet, "/page"}}, clientSideEndpoints...)
	for _, r := range requests {
		fetch(t, srv, r.method, r.path, "")
		fd := <-engine.seen
		if r.path == "/page" {
			if err := <-readDuringRequest; err != nil {
				t.Errorf("the page could not read the Element Data: %v", err)
			}
		}
		if _, err := fd.Get("device"); err == nil {
			t.Errorf("after %s %s the Flow Data still holds its Element "+
				"Data, so it was not closed", r.method, r.path)
		}
	}
}
