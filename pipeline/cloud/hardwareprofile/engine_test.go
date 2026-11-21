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

package hardwareprofile

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// noModelReason is the reason the stand-in cloud gives for a profile with no
// hardware model.
const noModelReason = "The model is not known for this profile."

// tacResponse is the partial JSON of a TAC lookup through the cloud service
// that the specification gives, with a fourth profile that has no model.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/json-builder.md?plain=1#L206-L237
const tacResponse = `{"hardware":{"profiles":[` +
	`{"hardwaremodel":"5217","hardwarename":["5217"],"hardwarevendor":"Coolpad"},` +
	`{"hardwaremodel":"5200","hardwarename":["5200"],"hardwarevendor":"Coolpad"},` +
	`{"hardwaremodel":"5310","hardwarename":["5310"],"hardwarevendor":"Coolpad"},` +
	`{"hardwaremodel":null,"hardwaremodelnullreason":"` + noModelReason + `",` +
	`"hardwarename":["Unknown"],"hardwarevendor":"Coolpad"}]}}`

// mockCloud stands in for the cloud service. Its hardware product has one top
// level Property, Profiles, with three sub-properties.
func mockCloud(response string) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Products":{"hardware":{"DataTier":"CloudV5Bespoke","Properties":[` +
			`{"Name":"Profiles","Type":"Array","Category":"Device","ItemProperties":[` +
			`{"Name":"HardwareModel","Type":"String","Category":"Device"},` +
			`{"Name":"HardwareName","Type":"Array","Category":"Device"},` +
			`{"Name":"HardwareVendor","Type":"String","Category":"Device"}]}]}}}`))
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["query.tac","query.nativemodel"]`))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	})
	return httptest.NewServer(mux)
}

// newRequestEngine builds a Cloud Request Engine against the stand-in cloud.
func newRequestEngine(t *testing.T, response string) *requestengine.CloudRequestEngine {
	t.Helper()
	srv := mockCloud(response)
	t.Cleanup(srv.Close)
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

// newPipeline builds a Pipeline of a Cloud Request Engine and the Hardware
// Profile Lookup Cloud Engine.
func newPipeline(t *testing.T, response string) (core.Pipeline, *Engine) {
	t.Helper()
	engine := New(nil)
	p, err := core.NewPipelineBuilder().
		AddFlowElement(newRequestEngine(t, response)).
		AddFlowElement(engine).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	return p, engine
}

// process looks up the TAC 35925406 and gives the engine's Element Data.
func process(t *testing.T, response string) *Data {
	t.Helper()
	p, _ := newPipeline(t, response)
	fd := p.CreateFlowData()
	fd.AddEvidence("query.tac", "35925406")
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	data, err := core.Get[*Data](fd)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// profiles gives the Profiles of data, failing the test when it has none.
func profiles(t *testing.T, data *Data) []*DeviceData {
	t.Helper()
	list, err := data.Profiles().Value()
	if err != nil {
		t.Fatalf("Profiles has no value: %v", err)
	}
	return list
}

// text gives the text value of a sub-property of a profile.
func text(t *testing.T, d *DeviceData, name string) engines.AspectPropertyValue[string] {
	t.Helper()
	v, err := d.Get(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	value, ok := v.(engines.AspectPropertyValue[string])
	if !ok {
		t.Fatalf("%s is a %T, want an AspectPropertyValue[string]", name, v)
	}
	return value
}

// When it is added to a Pipeline, this Engine initializes itself from a Cloud
// Request Engine, which MUST have been added to the Pipeline before it. The
// Hardware Profile Lookup Engine then takes the details of the subset of those
// Properties that are relevant to it.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/hardware-profile-lookup-cloud.md?plain=1#L36-L43
func TestPropertiesComeFromTheCloudRequestEngine(t *testing.T) {
	_, engine := newPipeline(t, tacResponse)
	if engine.DataKey() != DataKey || DataKey != "hardware" {
		t.Errorf("the data key is %q, want hardware", engine.DataKey())
	}
	var found *core.PropertyMetadata
	for _, p := range engine.Properties() {
		if strings.EqualFold(p.Name, "Profiles") {
			p := p
			found = &p
		}
	}
	if found == nil {
		t.Fatalf("the engine's Properties %v hold no Profiles", engine.Properties())
	}
	var items []string
	for _, item := range found.ItemProperties {
		items = append(items, item.Name)
	}
	want := []string{"HardwareModel", "HardwareName", "HardwareVendor"}
	if !reflect.DeepEqual(items, want) {
		t.Errorf("Profiles has the sub-properties %v, want %v", items, want)
	}
}

// The Cloud Request Engine MUST have been added to the Pipeline before this
// Engine, so a Pipeline without one before it is not built.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/hardware-profile-lookup-cloud.md?plain=1#L36-L38
func TestAPipelineWithoutACloudRequestEngineBeforeItIsNotBuilt(t *testing.T) {
	if _, err := core.NewPipelineBuilder().AddFlowElement(New(nil)).Build(); err == nil {
		t.Error("a Pipeline with no Cloud Request Engine was built")
	}
	_, err := core.NewPipelineBuilder().
		AddFlowElement(New(nil)).
		AddFlowElement(newRequestEngine(t, tacResponse)).
		Build()
	if err == nil {
		t.Error("a Pipeline with the Cloud Request Engine after this engine was built")
	}
}

// The Element Data holds profiles, a list of the devices whose TAC matches,
// one for each profile in the cloud response.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/hardware-profile-lookup-cloud.md?plain=1#L50-L52
func TestProfilesHoldEachDevice(t *testing.T) {
	list := profiles(t, process(t, tacResponse))
	if len(list) != 4 {
		t.Fatalf("there are %d profiles, want 4", len(list))
	}
	for i, model := range []string{"5217", "5200", "5310"} {
		got, err := text(t, list[i], "hardwaremodel").Value()
		if err != nil || got != model {
			t.Errorf("profile %d has the model %q, %v, want %s", i, got, err, model)
		}
	}
}

// Be aware that this engine also has sub-properties under the top level
// property that MUST also be catered for.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/hardware-profile-lookup-cloud.md?plain=1#L45-L46
func TestEachSubPropertyIsCateredFor(t *testing.T) {
	list := profiles(t, process(t, tacResponse))
	first := list[0]
	if vendor, err := text(t, first, "hardwarevendor").Value(); err != nil || vendor != "Coolpad" {
		t.Errorf("hardwarevendor is %q, %v, want Coolpad", vendor, err)
	}
	v, err := first.Get("hardwarename")
	if err != nil {
		t.Fatal(err)
	}
	names, ok := v.(engines.AspectPropertyValue[[]string])
	if !ok {
		t.Fatalf("hardwarename is a %T, want an AspectPropertyValue[[]string]", v)
	}
	if got, err := names.Value(); err != nil || !reflect.DeepEqual(got, []string{"5217"}) {
		t.Errorf("hardwarename is %v, %v, want [5217]", got, err)
	}
	// A sub-property is found whatever the case of its name.
	if _, err := first.Get("HardwareVendor"); err != nil {
		t.Errorf("HardwareVendor: %v", err)
	}
}

// A sub-property with no value has none, and gives the reason the cloud gave.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/hardware-profile-lookup-cloud.md?plain=1#L45-L46
func TestASubPropertyWithNoValueGivesItsReason(t *testing.T) {
	list := profiles(t, process(t, tacResponse))
	model := text(t, list[3], "hardwaremodel")
	if model.HasValue() {
		t.Error("the fourth profile has a model")
	}
	if model.NoValueMessage() != noModelReason {
		t.Errorf("the reason for no model is %q, want %q",
			model.NoValueMessage(), noModelReason)
	}
	if _, err := list[3].Get("hardwaremodelnullreason"); err == nil {
		t.Error("the reason is given as a sub-property of its own")
	}
}

// A lookup that matches no device gives an empty list of profiles.
func TestNoMatchGivesNoProfiles(t *testing.T) {
	list := profiles(t, process(t, `{"hardware":{"profiles":[]}}`))
	if len(list) != 0 {
		t.Errorf("there are %d profiles, want none", len(list))
	}
}
