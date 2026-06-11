package aspectengine

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/51Degrees/pipeline-go/cloud/requestengine"
	"github.com/51Degrees/pipeline-go/core"
)

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
