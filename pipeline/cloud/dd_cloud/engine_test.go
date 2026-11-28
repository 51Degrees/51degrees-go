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

package dd_cloud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// noVendorReason is the reason the stand-in cloud gives for having no
// hardware vendor.
const noVendorReason = "The hardware vendor needs a paid Resource Key."

// iPhone is the User-Agent the tests present.
const iPhone = "Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) " +
	"AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 Safari/9537.53"

// mockCloud stands in for the cloud service. Its Device Detection section
// gives a Boolean, a String, a list of Strings, and a Property with no value
// and a reason. The Resource Key also gives a location product, whose
// Property the Device Detection Cloud Engine leaves out.
func mockCloud() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Products":{"device":{"DataTier":"CloudV4Free","Properties":[` +
			`{"Name":"IsMobile","Type":"Boolean","Category":"Device"},` +
			`{"Name":"PlatformName","Type":"String","Category":"Software"},` +
			`{"Name":"HardwareName","Type":"Array","Category":"Device"},` +
			`{"Name":"HardwareVendor","Type":"String","Category":"Device"}]},` +
			`"location":{"DataTier":"CloudV4Free","Properties":[` +
			`{"Name":"Town","Type":"String","Category":"Location"}]}}}`))
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["header.user-agent"]`))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"device":{` +
			`"ismobile":true,` +
			`"platformname":"iOS",` +
			`"hardwarename":["iPhone","iPhone 5S"],` +
			`"hardwarevendor":null,"hardwarevendornullreason":"` + noVendorReason + `"}}`))
	})
	return httptest.NewServer(mux)
}

// requestEngine gives a Cloud Request Engine that calls the stand-in cloud.
func requestEngine(t *testing.T, srv *httptest.Server) *requestengine.CloudRequestEngine {
	cre, err := requestengine.NewBuilder().
		SetResourceKey("test-resource-key").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	return cre
}

// process runs the iPhone User-Agent through a Pipeline of a Cloud Request
// Engine and then the Device Detection Cloud Engine, against the stand-in
// cloud.
func process(t *testing.T) *Data {
	srv := mockCloud()
	t.Cleanup(srv.Close)
	pipeline, err := core.NewPipelineBuilder().
		AddFlowElement(requestEngine(t, srv)).
		AddFlowElement(New(nil)).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pipeline.Close() })
	fd := pipeline.CreateFlowData()
	fd.AddEvidence("header.user-agent", iPhone)
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	data, err := core.Get[*Data](fd)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// text gives a Property that is not a list as its text.
func text(t *testing.T, data *Data, name string) string {
	t.Helper()
	value, err := data.Get(name)
	if err != nil {
		t.Fatal(err)
	}
	got, err := value.(engines.AspectPropertyValue[string]).Value()
	if err != nil {
		t.Fatalf("%s has no value: %v", name, err)
	}
	return got
}

// The Device Detection Cloud Engine, added to a Pipeline after a Cloud
// Request Engine, initializes itself from that engine and gives the Device
// Detection Properties of the cloud response.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/device-detection-cloud.md?plain=1#L22-L24
func TestEngineFollowsACloudRequestEngine(t *testing.T) {
	data := process(t)
	if got := text(t, data, "ismobile"); got != "true" {
		t.Errorf("ismobile is %q, want true", got)
	}
	if got := text(t, data, "platformname"); got != "iOS" {
		t.Errorf("platformname is %q, want iOS", got)
	}
	value, err := data.Get("hardwarename")
	if err != nil {
		t.Fatal(err)
	}
	names, err := value.(engines.AspectPropertyValue[[]string]).Value()
	if err != nil || len(names) != 2 || names[0] != "iPhone" || names[1] != "iPhone 5S" {
		t.Errorf("hardwarename is %v, %v, want iPhone then iPhone 5S", names, err)
	}
}

// A Property with no value has none, and gives the reason the cloud gave.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/cloud-aspect-engine.md
func TestNoValue(t *testing.T) {
	value, err := process(t).Get("hardwarevendor")
	if err != nil {
		t.Fatal(err)
	}
	vendor := value.(engines.AspectPropertyValue[string])
	if vendor.HasValue() {
		t.Error("hardwarevendor has a value")
	}
	if vendor.NoValueMessage() != noVendorReason {
		t.Errorf("the reason for no hardwarevendor is %q", vendor.NoValueMessage())
	}
}

// The engine takes the Device Detection subset of the Properties the Cloud
// Request Engine found for the Resource Key.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/device-detection-cloud.md?plain=1#L26-L29
func TestPropertiesAreTheDeviceDetectionSubset(t *testing.T) {
	srv := mockCloud()
	defer srv.Close()
	engine := New(nil)
	pipeline, err := core.NewPipelineBuilder().
		AddFlowElement(requestEngine(t, srv)).
		AddFlowElement(engine).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	defer pipeline.Close()
	names := map[string]bool{}
	for _, p := range engine.Properties() {
		names[p.Name] = true
	}
	for _, want := range []string{"IsMobile", "PlatformName", "HardwareName", "HardwareVendor"} {
		if !names[want] {
			t.Errorf("the engine's Properties lack %s, got %v", want, names)
		}
	}
	if names["Town"] {
		t.Errorf("the engine's Properties hold Town, from the location product, got %v", names)
	}
}

// The engine initializes itself from a Cloud Request Engine added to the
// Pipeline before it, so a Pipeline with none, or with one only after the
// engine, is not built.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/device-detection-cloud.md?plain=1#L22-L24
func TestAPipelineWithoutACloudRequestEngineBeforeItIsNotBuilt(t *testing.T) {
	if p, err := core.NewPipelineBuilder().AddFlowElement(New(nil)).Build(); err == nil {
		p.Close()
		t.Error("a Pipeline with no Cloud Request Engine was built")
	}
	srv := mockCloud()
	defer srv.Close()
	p, err := core.NewPipelineBuilder().
		AddFlowElement(New(nil)).
		AddFlowElement(requestEngine(t, srv)).
		Build()
	if err == nil {
		p.Close()
		t.Error("a Pipeline with the Cloud Request Engine after this engine was built")
	}
}

// A Pipeline holding the engine can be built from a configuration file by
// registering Factory beside the Cloud Request Engine's.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/pipeline-configuration.md?plain=1#L8-L10
func TestBuiltFromConfiguration(t *testing.T) {
	srv := mockCloud()
	defer srv.Close()
	registry := core.NewBuilderRegistry()
	registry.Register("CloudRequestEngine", requestengine.Factory)
	registry.Register("DeviceDetectionCloudEngine", Factory)
	file := `{"PipelineOptions": {"Elements": [
		{"BuilderName": "CloudRequestEngine", "BuildParameters": {
			"ResourceKey": "test-resource-key", "EndPoint": "` + srv.URL + `/"}},
		{"BuilderName": "DeviceDetectionCloudEngine"}]}}`
	var options core.PipelineOptions
	if err := json.Unmarshal([]byte(file), &options); err != nil {
		t.Fatal(err)
	}
	pipeline, err := registry.BuildFromConfiguration(options, nil)
	if err != nil {
		t.Fatalf("building from configuration gave %v", err)
	}
	defer pipeline.Close()
	fd := pipeline.CreateFlowData()
	fd.AddEvidence("header.user-agent", iPhone)
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	data, err := core.Get[*Data](fd)
	if err != nil {
		t.Fatal(err)
	}
	if got := text(t, data, "platformname"); got != "iOS" {
		t.Errorf("platformname is %q, want iOS", got)
	}
}
