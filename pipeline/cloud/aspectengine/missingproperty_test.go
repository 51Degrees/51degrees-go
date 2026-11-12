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
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/missingproperty"
)

// These tests follow "Deciding the reason" in the missing Properties section
// of the Pipeline specification, for a Cloud Aspect Engine in a Pipeline.
// See https://github.com/51Degrees/specifications/blob/a0a7008a6875b1b8aae4f947acd5252466c777fe/pipeline-specification/features/properties.md#deciding-the-reason

// keepingEngine is a Cloud Aspect Engine that adds its Element Data whether
// or not the Cloud Request Engine has a response.
type keepingEngine struct {
	*BaseCloudAspectEngine
}

func (e *keepingEngine) Process(fd core.FlowData) error {
	data := engines.NewBaseAspectData()
	if aspect, err := ParseAspect(fd, e.DataKey()); err == nil {
		for name := range aspect {
			data.Set(name, PropertyValue[json.RawMessage](aspect, name))
		}
	}
	fd.GetOrAdd(e.DataKey(), func(core.FlowData) core.ElementData {
		return data
	})
	return nil
}

// quietLogger discards what it is given.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// cloudPipeline builds a Pipeline of a Cloud Request Engine against srv and
// engine, with process errors suppressed.
func cloudPipeline(t *testing.T, srv *httptest.Server,
	engine core.FlowElement) core.Pipeline {
	t.Helper()
	re, err := requestengine.NewBuilder().
		SetResourceKey("k").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		SetLogger(quietLogger()).
		Build()
	if err != nil {
		t.Fatalf("build the request engine: %v", err)
	}
	p, err := core.NewPipelineBuilder().
		SetLogger(quietLogger()).
		SetSuppressProcessExceptions(true).
		AddFlowElement(re).
		AddFlowElement(engine).
		Build()
	if err != nil {
		t.Fatalf("build the Pipeline: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

// processed gives the Element Data under key after one request through p.
func processed(t *testing.T, p core.Pipeline, key string) core.ElementData {
	t.Helper()
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, err := fd.Get(key)
	if err != nil {
		t.Fatalf("no Element Data under %s: %v", key, err)
	}
	return data
}

// checkMissing reads name from data and compares the error with want, which
// one of the missingproperty builders gives.
func checkMissing(t *testing.T, data core.ElementData, name string,
	want *core.PropertyMissingError) {
	t.Helper()
	_, err := data.Get(name)
	var got *core.PropertyMissingError
	if !errors.As(err, &got) {
		t.Fatalf("reading %s gives %v, want a PropertyMissingError", name, err)
	}
	if got.Reason != want.Reason || got.Error() != want.Error() {
		t.Errorf("reading %s\n got: %s, %s\nwant: %s, %s", name,
			got.Reason, got.Error(), want.Reason, want.Error())
	}
}

// A Resource Key that includes the engine's product and not the Property
// gives PropertyNotAccessibleWithResourceKey, naming the Properties the key
// does include.
func TestPropertyNotInTheResourceKey(t *testing.T) {
	srv := httptest.NewServer(fakeCloud())
	t.Cleanup(srv.Close)
	p := cloudPipeline(t, srv,
		&sectionEngine{NewBaseCloudAspectEngine("device", quietLogger())})
	checkMissing(t, processed(t, p, "device"), "ScreenWidth",
		missingproperty.ForPropertyNotInResourceKey("ScreenWidth", "device",
			[]string{"ismobile", "hardwarevendor"}))
}

// A Resource Key that includes no product for the engine, or lists the
// product with no Properties, gives ProductNotAccessibleWithResourceKey.
func TestProductNotInTheResourceKey(t *testing.T) {
	clouds := map[string]func() http.Handler{
		"not listed":                fakeCloud,
		"listed with no Properties": emptyProductCloud,
	}
	for name, cloud := range clouds {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(cloud())
			t.Cleanup(srv.Close)
			p := cloudPipeline(t, srv, &sectionEngine{
				NewBaseCloudAspectEngine("location", quietLogger())})
			checkMissing(t, processed(t, p, "location"), "Country",
				missingproperty.ForProductNotInResourceKey("Country",
					"location"))
		})
	}
}

// When the Cloud Request Engine has no response for the Flow Data because
// its request failed, every missing Property gives CloudRequestFailed,
// whatever the engine's metadata says.
func TestFailedCloudRequestIsTheReason(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/accessibleproperties", fakeCloud())
	mux.Handle("/evidencekeys", fakeCloud())
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	p := cloudPipeline(t, srv,
		&keepingEngine{NewBaseCloudAspectEngine("device", quietLogger())})
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	_ = fd.Process() // the failed request is suppressed
	data, err := fd.Get("device")
	if err != nil {
		t.Fatalf("no Element Data under device: %v", err)
	}
	checkMissing(t, data, "IsMobile",
		missingproperty.ForCloudRequestFailed("IsMobile", "device"))
	checkMissing(t, data, "ScreenWidth",
		missingproperty.ForCloudRequestFailed("ScreenWidth", "device"))
}

// A Cloud Aspect Engine says its request for the property list failed from
// the failure until the list is fetched.
func TestPropertyListRequestFailedUntilTheListIsFetched(t *testing.T) {
	var healthy atomic.Bool
	srv := switchedCloud(t, &healthy)
	engine := &sectionEngine{NewBaseCloudAspectEngine("device", quietLogger())}
	p := cloudPipeline(t, srv, engine)
	state, ok := any(engine).(missingproperty.CloudEngine)
	if !ok {
		t.Fatal("the Cloud Aspect Engine is not a missingproperty.CloudEngine")
	}
	if !state.PropertyListRequestFailed() {
		t.Error("the property list request failed when the engine was " +
			"built, and the engine does not say so")
	}
	healthy.Store(true)
	checkMissing(t, processed(t, p, "device"), "ScreenWidth",
		missingproperty.ForPropertyNotInResourceKey("ScreenWidth", "device",
			[]string{"ismobile", "hardwarevendor"}))
	if state.PropertyListRequestFailed() {
		t.Error("the property list was fetched, and the engine still says " +
			"its request failed")
	}
}
