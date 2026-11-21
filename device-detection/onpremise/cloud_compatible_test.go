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

package onpremise

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/dd_cloud"
	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// cloudAnswer stands in for the cloud service, answering for the iPhone as
// the Lite data file does, in the form the cloud gives a Boolean, a String
// and a list of Strings.
func cloudAnswer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Products":{"device":{"DataTier":"CloudV4Free","Properties":[` +
			`{"Name":"IsMobile","Type":"Boolean","Category":"Device"},` +
			`{"Name":"PlatformName","Type":"String","Category":"Software"},` +
			`{"Name":"BrowserName","Type":"String","Category":"Browser"}]}}}`))
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["header.user-agent"]`))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"device":{"ismobile":true,"platformname":"iOS",` +
			`"browsername":"Mobile Safari"}}`))
	})
	return httptest.NewServer(mux)
}

// cloudData gives the Device Detection Cloud Engine's Element Data for the
// iPhone, from the stand-in cloud.
func cloudData(t *testing.T) *dd_cloud.Data {
	srv := cloudAnswer()
	t.Cleanup(srv.Close)
	cre, err := requestengine.NewBuilder().
		SetResourceKey("test-resource-key").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := core.NewPipelineBuilder().
		AddFlowElement(cre).
		AddFlowElement(dd_cloud.New(nil)).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pipeline.Close() })
	fd := pipeline.CreateFlowData()
	fd.AddEvidence("header.user-agent", aspectIPhone)
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	data, err := core.Get[*dd_cloud.Data](fd)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// onPremiseData gives the on-premise engine's Element Data for the iPhone,
// from a Pipeline.
func onPremiseData(t *testing.T) *DeviceData {
	engine, err := NewPipelineEngine(WithDataFile(aspectDataFile), WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := core.NewPipelineBuilder().AddFlowElement(engine).Build()
	if err != nil {
		engine.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { pipeline.Close() })
	fd := pipeline.CreateFlowData()
	fd.AddEvidence("header.user-agent", aspectIPhone)
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	data, err := core.Get[*DeviceData](fd)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The on-premise engine and the Device Detection Cloud Engine use the same
// Data Key, and give each Property they share as the same type, written the
// same way, so code reading one reads the other.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/cloud-aspect-engine.md?plain=1#L24-L26
func TestElementDataIsCompatibleWithTheCloudEngines(t *testing.T) {
	if DataKey != dd_cloud.DataKey {
		t.Errorf("the on-premise Data Key is %q and the cloud one %q",
			DataKey, dd_cloud.DataKey)
	}
	cloud, onPremise := cloudData(t), onPremiseData(t)
	for _, name := range []string{"ismobile", "platformname", "browsername"} {
		want, err := cloud.Get(name)
		if err != nil {
			t.Fatalf("the cloud gives no %s: %v", name, err)
		}
		got, err := onPremise.Get(name)
		if err != nil {
			t.Errorf("the on-premise engine gives no %s: %v", name, err)
			continue
		}
		if fmt.Sprintf("%T", got) != fmt.Sprintf("%T", want) {
			t.Errorf("%s is a %T on-premise and a %T from the cloud", name, got, want)
			continue
		}
		cloudValue := want.(engines.PropertyValueAccessor).AsInterface()
		onPremiseValue := got.(engines.PropertyValueAccessor).AsInterface()
		if cloudValue != onPremiseValue {
			t.Errorf("%s is %v on-premise and %v from the cloud",
				name, onPremiseValue, cloudValue)
		}
	}
}

// Each list Property the on-premise engine gives is a list of Strings, the
// type the cloud gives a list in.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/cloud-aspect-engine.md?plain=1#L24-L26
func TestListPropertiesAreListsOfStrings(t *testing.T) {
	data := onPremiseData(t)
	engine, err := New(WithDataFile(aspectDataFile), WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Stop()
	properties, err := engine.Properties()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range properties {
		if !p.Available {
			continue
		}
		value, err := data.Get(p.Name)
		if err != nil {
			t.Errorf("%s is missing: %v", p.Name, err)
			continue
		}
		_, isList := value.(engines.AspectPropertyValue[[]string])
		_, isText := value.(engines.AspectPropertyValue[string])
		if p.IsList && !isList || !p.IsList && !isText {
			t.Errorf("%s, a list %v, is a %T", p.Name, p.IsList, value)
		}
	}
}
