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

package aspectengine

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// TestResponseJSONDistinguishesFailure verifies that when the cloud request ran
// but failed (under suppressed exceptions), ResponseJSON reports a failure
// rather than the "engine must run first" message used when it never ran.
func TestResponseJSONDistinguishesFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Products":{}}`))
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errors":["boom"]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	re, err := requestengine.NewBuilder().
		SetResourceKey("k").SetEndpoint(srv.URL + "/").SetHTTPClient(srv.Client()).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p, _ := core.NewPipelineBuilder().SetSuppressProcessExceptions(true).AddFlowElement(re).Build()
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	_ = fd.Process() // failure is suppressed but a failed marker is stored

	_, err = ResponseJSON(fd)
	if err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("expected a cloud-request-failed error, got %v", err)
	}
}

func fakeCloud() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Products":{"device":{"DataTier":"CloudV4","Properties":[{"Name":"ismobile","Type":"Bool","Category":"Device"},{"Name":"hardwarevendor","Type":"String","Category":"Device"}]}}}`))
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["header.user-agent"]`))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"device":{"ismobile":true,"hardwarevendor":null,"hardwarevendornullreason":"Requires more evidence"}}`))
	})
	return mux
}

func processedFlowData(t *testing.T) (core.Pipeline, core.FlowData) {
	t.Helper()
	srv := httptest.NewServer(fakeCloud())
	t.Cleanup(srv.Close)
	re, err := requestengine.NewBuilder().
		SetResourceKey("k").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p, _ := core.NewPipelineBuilder().AddFlowElement(re).Build()
	t.Cleanup(func() { p.Close() })
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	return p, fd
}

func TestParseAspectAndPropertyValue(t *testing.T) {
	p, fd := processedFlowData(t)
	defer p.Close()

	aspect, err := ParseAspect(fd, "device")
	if err != nil {
		t.Fatalf("parse aspect: %v", err)
	}

	isMobile := PropertyValue[bool](aspect, "ismobile")
	if !isMobile.HasValue() {
		t.Fatal("ismobile should have a value")
	}
	if v, _ := isMobile.Value(); v != true {
		t.Fatalf("ismobile = %v", v)
	}

	// Null value with a nullreason becomes a no-value with that message.
	vendor := PropertyValue[string](aspect, "hardwarevendor")
	if vendor.HasValue() {
		t.Fatal("hardwarevendor should have no value")
	}
	if vendor.NoValueMessage() != "Requires more evidence" {
		t.Fatalf("nullreason not applied: %q", vendor.NoValueMessage())
	}
}

func TestAspectPropertiesFromRequestEngine(t *testing.T) {
	p, fd := processedFlowData(t)
	defer p.Close()

	props, err := AspectProperties(fd, nil, "device")
	if err != nil {
		t.Fatalf("aspect properties: %v", err)
	}
	if len(props) != 2 {
		t.Fatalf("expected 2 properties, got %d", len(props))
	}
	names := map[string]bool{}
	for _, pm := range props {
		names[pm.Name] = true
	}
	if !names["ismobile"] || !names["hardwarevendor"] {
		t.Fatalf("unexpected properties: %+v", props)
	}
}

func TestMissingCloudResponse(t *testing.T) {
	// A pipeline with no CloudRequestEngine should yield a clear error.
	p, _ := core.NewPipelineBuilder().Build()
	defer p.Close()
	fd := p.CreateFlowData()
	_ = fd.Process()
	if _, err := ParseAspect(fd, "device"); err == nil {
		t.Fatal("expected error when cloud response is missing")
	}
}
