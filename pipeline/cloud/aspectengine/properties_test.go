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

// These tests follow the rule that consumers of the Cloud Request Engine's
// Properties see them once they are known.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#consequences-for-consumers-of-the-engine

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// switchedCloud is fakeCloud whose discovery endpoints answer 503 until it
// is healthy.
func switchedCloud(t *testing.T, healthy *atomic.Bool) *httptest.Server {
	t.Helper()
	cloud := fakeCloud()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !healthy.Load() && r.URL.Path != "/json" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		cloud.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// devicePipeline builds a Pipeline of a Cloud Request Engine against srv and
// a Cloud Aspect Engine for device.
func devicePipeline(t *testing.T, srv *httptest.Server) core.Pipeline {
	t.Helper()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	re, err := requestengine.NewBuilder().
		SetResourceKey("k").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		SetLogger(quiet).
		Build()
	if err != nil {
		t.Fatalf("build the request engine: %v", err)
	}
	p, err := core.NewPipelineBuilder().
		SetLogger(quiet).
		SetSuppressProcessExceptions(true).
		AddFlowElement(re).
		AddFlowElement(&sectionEngine{NewBaseCloudAspectEngine("device", quiet)}).
		Build()
	if err != nil {
		t.Fatalf("build the Pipeline: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

// deviceProperties returns the sorted names of the Pipeline's Properties
// that belong to the device engine.
func deviceProperties(p core.Pipeline) []string {
	var names []string
	for _, pm := range p.Properties() {
		if pm.Element != nil && pm.Element.DataKey() == "device" {
			names = append(names, pm.Name)
		}
	}
	sort.Strings(names)
	return names
}

// A Cloud Aspect Engine gives the Properties of its product as the Cloud
// Request Engine found them when it was built, so the Pipeline's Property
// metadata holds them before any request.
func TestPipelinePropertiesHoldCloudProperties(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(true)
	p := devicePipeline(t, switchedCloud(t, &healthy))
	if got := deviceProperties(p); len(got) != 2 ||
		got[0] != "hardwarevendor" || got[1] != "ismobile" {
		t.Errorf("the Pipeline's device Properties are %v, want "+
			"[hardwarevendor ismobile]", got)
	}
}

// After a transient failure when the Cloud Request Engine is built, the
// Pipeline's Property metadata has no cloud Properties, and holds them once
// a request has fetched them, because the Pipeline gathers the metadata
// again rather than keeping what it found when it was built.
func TestPipelinePropertiesFollowRetriedDiscovery(t *testing.T) {
	var healthy atomic.Bool
	p := devicePipeline(t, switchedCloud(t, &healthy))
	if got := deviceProperties(p); len(got) != 0 {
		t.Fatalf("the Pipeline has the device Properties %v before they "+
			"are known", got)
	}
	healthy.Store(true)
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	if err := fd.Process(); err != nil || fd.HasErrors() {
		t.Fatalf("process: %v %v", err, fd.Errors())
	}
	if got := deviceProperties(p); len(got) != 2 {
		t.Errorf("the Pipeline's device Properties are %v once fetched, "+
			"want [hardwarevendor ismobile]", got)
	}
}

// metadataCloud is a stand-in cloud service whose device product has a
// Property that delays execution and one that lists it as Evidence, with
// item Properties.
func metadataCloud() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"Products":{"device":{"DataTier":"CloudV4",`+
			`"Properties":[`+
			`{"Name":"javascriptprofile","Type":"JavaScript",`+
			`"Category":"Device","DelayExecution":true},`+
			`{"Name":"profiles","Type":"Array","Category":"Device",`+
			`"EvidenceProperties":["device.javascriptprofile"],`+
			`"ItemProperties":[{"Name":"profileid","Type":"String",`+
			`"Category":"Device"}]}]}}}`)
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `["header.user-agent"]`)
	})
	return mux
}

// A cloud Property's metadata carries what the cloud service gives for it,
// being whether it delays execution, the Properties that are its Evidence
// and its item Properties, so the JSON Builder can write delayexecution and
// evidenceproperties for it as .NET does.
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/pipeline-elements/json-builder.md
func TestCloudPropertiesCarryTheirMetadata(t *testing.T) {
	srv := httptest.NewServer(metadataCloud())
	t.Cleanup(srv.Close)
	p := devicePipeline(t, srv)
	byName := map[string]core.PropertyMetadata{}
	for _, pm := range p.Properties() {
		if pm.Element != nil && pm.Element.DataKey() == "device" {
			byName[pm.Name] = pm
		}
	}
	js, profiles := byName["javascriptprofile"], byName["profiles"]
	if !js.DelayExecution {
		t.Errorf("javascriptprofile does not delay execution: %+v", js)
	}
	if profiles.DelayExecution {
		t.Errorf("profiles delays execution: %+v", profiles)
	}
	if !reflect.DeepEqual(profiles.EvidenceProperties,
		[]string{"device.javascriptprofile"}) {
		t.Errorf("profiles has the Evidence Properties %v, want "+
			"[device.javascriptprofile]", profiles.EvidenceProperties)
	}
	if len(profiles.ItemProperties) != 1 {
		t.Fatalf("profiles has the item Properties %+v, want profileid",
			profiles.ItemProperties)
	}
	item := profiles.ItemProperties[0]
	if item.Name != "profileid" || item.Category != "Device" ||
		!item.Available || item.Element != profiles.Element {
		t.Errorf("the item Property is %+v, want profileid of the device "+
			"engine", item)
	}
}

// A Cloud Aspect Engine in a parallel group finds itself and the Cloud
// Request Engine, so it gives its Properties and reports them loaded, when
// discovery succeeds at build and once a request has fetched them after a
// transient failure.
func TestCloudPropertiesInAParallelGroup(t *testing.T) {
	for _, healthyAtBuild := range []bool{true, false} {
		t.Run(fmt.Sprintf("healthy at build=%v", healthyAtBuild), func(t *testing.T) {
			var healthy atomic.Bool
			healthy.Store(healthyAtBuild)
			srv := switchedCloud(t, &healthy)
			quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
			re, err := requestengine.NewBuilder().
				SetResourceKey("k").
				SetEndpoint(srv.URL + "/").
				SetHTTPClient(srv.Client()).
				SetLogger(quiet).
				Build()
			if err != nil {
				t.Fatalf("build the request engine: %v", err)
			}
			engine := &sectionEngine{NewBaseCloudAspectEngine("device", quiet)}
			p, err := core.NewPipelineBuilder().
				SetLogger(quiet).
				SetSuppressProcessExceptions(true).
				AddFlowElement(re).
				AddFlowElement(core.NewParallelElements(quiet, engine)).
				Build()
			if err != nil {
				t.Fatalf("build the Pipeline: %v", err)
			}
			t.Cleanup(func() { p.Close() })

			if !healthyAtBuild {
				if got := deviceProperties(p); len(got) != 0 || engine.PropertiesLoaded() {
					t.Fatalf("before discovery the Properties are %v and "+
						"loaded is %v, want none and false", got,
						engine.PropertiesLoaded())
				}
				healthy.Store(true)
				fd := p.CreateFlowData()
				fd.AddEvidence("header.user-agent", "ua")
				if err := fd.Process(); err != nil || fd.HasErrors() {
					t.Fatalf("process: %v %v", err, fd.Errors())
				}
			}
			if got := deviceProperties(p); len(got) != 2 {
				t.Errorf("the Pipeline's device Properties are %v, want "+
					"[hardwarevendor ismobile]", got)
			}
			for _, pm := range p.Properties() {
				if pm.Name == "ismobile" && pm.Element != core.FlowElement(engine) {
					t.Errorf("ismobile names the element %v, want the engine",
						pm.Element)
				}
			}
			if !engine.PropertiesLoaded() {
				t.Error("the engine does not report its Properties loaded")
			}
		})
	}
}
