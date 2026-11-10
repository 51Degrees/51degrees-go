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

package requestengine

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// The request carries only the Evidence the cloud service accepts, with each
// prefix removed and the query value winning a conflict. Evidence that strips
// to resource, license, licence or values is left out even when the service
// lists those keys, because the engine writes those fields itself, the
// service reads licence as the license key, and a second field would leave
// which applied undecided.
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/pipeline-elements/cloud-request-engine.md#processing
func TestRequestCarriesOnlyEvidenceTheServiceNeeds(t *testing.T) {
	var mu sync.Mutex
	var form url.Values
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"Products":{}}`)
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `["header.user-agent","query.user-agent",`+
			`"query.resource","header.resource","query.license",`+
			`"query.licence","query.values"]`)
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		form = r.PostForm
		mu.Unlock()
		io.WriteString(w, `{"device":{}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	e, err := NewBuilder().
		SetResourceKey("configured-key").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build the Pipeline: %v", err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	for key, value := range map[string]string{
		"header.user-agent":      "header agent",
		"query.user-agent":       "query agent",
		"header.accept-language": "en-GB",
		"query.resource":         "evidence-resource",
		"header.resource":        "header-resource",
		"query.license":          "evidence-license",
		"query.licence":          "evidence-licence",
		"query.values":           "device.ismobile",
	} {
		fd.AddEvidence(key, value)
	}
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := url.Values{
		"resource":   {"configured-key"},
		"user-agent": {"query agent"},
	}
	if !reflect.DeepEqual(form, want) {
		t.Errorf("the request carried %v, want %v", form, want)
	}
}

// Evidence named in any letter case is matched to the keys the cloud service
// accepts, also in any letter case, and Evidence that strips to resource,
// license, licence or values in any letter case is left out.
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/pipeline-elements/cloud-request-engine.md#processing
func TestRequestLeavesOutEngineFieldsInAnyLetterCase(t *testing.T) {
	var mu sync.Mutex
	var form url.Values
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"Products":{}}`)
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `["Header.User-Agent","query.LICENCE",`+
			`"header.Resource","Query.Values","query.License"]`)
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		form = r.PostForm
		mu.Unlock()
		io.WriteString(w, `{"device":{}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	e, err := NewBuilder().
		SetResourceKey("configured-key").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build the Pipeline: %v", err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	for key, value := range map[string]string{
		"header.User-Agent": "header agent",
		"query.LICENCE":     "evidence-licence",
		"header.Resource":   "header-resource",
		"QUERY.values":      "device.ismobile",
		"query.LiCeNsE":     "evidence-license",
	} {
		fd.AddEvidence(key, value)
	}
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := url.Values{
		"resource":   {"configured-key"},
		"user-agent": {"header agent"},
	}
	if !reflect.DeepEqual(form, want) {
		t.Errorf("the request carried %v, want %v", form, want)
	}
}
