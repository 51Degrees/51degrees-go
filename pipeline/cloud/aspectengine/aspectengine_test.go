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
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
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

// recordingHandler keeps every log record made through it.
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

func (h *recordingHandler) all() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]slog.Record(nil), h.records...)
}

// sectionEngine is a Cloud Aspect Engine that copies its section of the
// cloud response into its Element Data, as the IP Intelligence cloud engine
// does.
type sectionEngine struct {
	*BaseCloudAspectEngine
}

func (e *sectionEngine) Process(fd core.FlowData) error {
	aspect, err := ParseAspect(fd, e.DataKey())
	if err != nil {
		return err
	}
	data := engines.NewBaseAspectData()
	for name := range aspect {
		data.Set(name, PropertyValue[json.RawMessage](aspect, name))
	}
	fd.GetOrAdd(e.DataKey(), func(core.FlowData) core.ElementData {
		return data
	})
	return nil
}

// buildWithLogger builds a Pipeline of a Cloud Request Engine against the
// stand-in cloud and a sectionEngine for aspectKey, all logging to log.
func buildWithLogger(t *testing.T, cloud http.Handler, aspectKey string,
	suppress bool, log *slog.Logger) (core.Pipeline, *sectionEngine) {
	t.Helper()
	srv := httptest.NewServer(cloud)
	t.Cleanup(srv.Close)
	re, err := requestengine.NewBuilder().
		SetResourceKey("k").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		SetLogger(log).
		Build()
	if err != nil {
		t.Fatalf("build the request engine: %v", err)
	}
	engine := &sectionEngine{NewBaseCloudAspectEngine(aspectKey, log)}
	p, err := core.NewPipelineBuilder().
		SetLogger(log).
		SetSuppressProcessExceptions(suppress).
		AddFlowElement(re).
		AddFlowElement(engine).
		Build()
	if err != nil {
		t.Fatalf("build the Pipeline: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p, engine
}

// emptyProductCloud is fakeCloud with "location" listed as a product that
// has no Properties, and no "location" section in the response.
func emptyProductCloud() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Products":{"device":{"DataTier":"CloudV4","Properties":[{"Name":"ismobile","Type":"Bool","Category":"Device"}]},"location":{"DataTier":"CloudV4","Properties":[]}}}`))
	})
	mux.Handle("/evidencekeys", fakeCloud())
	mux.Handle("/json", fakeCloud())
	return mux
}

// A Resource Key that includes no product for a Cloud Aspect Engine, or
// lists the product with no Properties, fails neither the Pipeline build
// nor any request. The engine has no Property metadata and logs the missing
// product once, at error level with a link to the Configurator, with
// nothing logged on each request. Its Element Data is empty, so reading a
// Property gives the error that reading any Property the response does not
// carry gives. Each case builds a new engine, which logs once of its own.
func TestMissingProductBuildsAndLogsOnce(t *testing.T) {
	clouds := []struct {
		name  string
		cloud func() http.Handler
	}{
		{"not listed", fakeCloud}, // fakeCloud's key includes only "device"
		{"listed with no Properties", emptyProductCloud},
	}
	for _, c := range clouds {
		for _, suppress := range []bool{false, true} {
			name := fmt.Sprintf("%s/suppress=%v", c.name, suppress)
			t.Run(name, func(t *testing.T) {
				checkMissingProduct(t, c.cloud(), suppress)
			})
		}
	}
}

// checkMissingProduct runs three requests through a Pipeline whose key gives
// the "location" engine no Properties, and checks the results and the log.
func checkMissingProduct(t *testing.T, cloud http.Handler, suppress bool) {
	const missing = "location"
	log := &recordingHandler{}
	p, engine := buildWithLogger(t, cloud, missing, suppress, slog.New(log))
	_, absent := engines.NewBaseAspectData().Get("town")
	for i := 1; i <= 3; i++ {
		fd := p.CreateFlowData()
		fd.AddEvidence("header.user-agent", "ua")
		if err := fd.Process(); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if errs := fd.Errors(); len(errs) != 0 {
			t.Fatalf("request %d recorded errors: %v", i, errs)
		}
		if i == 1 {
			props, err := AspectProperties(fd, engine, missing)
			if err != nil || len(props) != 0 {
				t.Errorf("the engine's Properties are %v, %v, want none",
					props, err)
			}
		}
		data, err := fd.Get(missing)
		if err != nil {
			t.Fatalf("request %d has no Element Data: %v", i, err)
		}
		if values := data.AsMap(); len(values) != 0 {
			t.Errorf("request %d has the values %v, want none", i, values)
		}
		_, err = data.Get("town")
		if err == nil || err.Error() != absent.Error() {
			t.Errorf("reading town gives %v, want %v", err, absent)
		}
	}
	records := log.all()
	if len(records) != 1 {
		for _, r := range records {
			t.Logf("%s %s", r.Level, r.Message)
		}
		t.Fatalf("logged %d times over 3 requests, want once", len(records))
	}
	product, configurator := "", ""
	records[0].Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "product":
			product = a.Value.String()
		case "configurator":
			configurator = a.Value.String()
		}
		return true
	})
	if records[0].Level != slog.LevelError {
		t.Errorf("logged at %s, want %s", records[0].Level, slog.LevelError)
	}
	if product != missing {
		t.Errorf("the log names the product %q, want %q", product, missing)
	}
	if configurator != configuratorURL {
		t.Errorf("the log links to %q, want %q", configurator,
			configuratorURL)
	}
}

// A response with no section for a product the Resource Key includes is
// still an error, and is not logged as a missing product.
func TestAbsentSectionOfIncludedProductIsAnError(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/accessibleproperties", fakeCloud())
	mux.Handle("/evidencekeys", fakeCloud())
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"other":{}}`))
	})
	log := &recordingHandler{}
	p, _ := buildWithLogger(t, mux, "device", false, slog.New(log))
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	if err := fd.Process(); err == nil {
		t.Fatal("a response with no section for an included product " +
			"gives no error")
	}
	if records := log.all(); len(records) != 0 {
		t.Errorf("logged %q, want nothing", records[0].Message)
	}
}
