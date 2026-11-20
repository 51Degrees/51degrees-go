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

package ipi_onpremise

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_cloud"
	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// ipData is what code written for either Engine reads its results through.
// It is Aspect Data, read by a Property's name.
type ipData interface {
	engines.AspectData
}

// Both Element Data types are read through the one interface.
var (
	_ ipData = (*AspectData)(nil)
	_ ipData = (*ipi_cloud.Data)(nil)
)

// cloudNoValueReason is the reason the stand-in cloud gives for a Property
// with no value.
const cloudNoValueReason = "The stand-in cloud holds no value."

// weightedValueJSON is a weighted value as the cloud service writes it.
type weightedValueJSON struct {
	Value        string `json:"value"`
	RawWeighting uint16 `json:"rawweighting"`
}

// standInCloud stands in for the cloud service. It gives the named Property
// the given values as a weighted Property, and gives the Property named
// without no value and a reason.
func standInCloud(t *testing.T, name string, values []core.WeightedValue[string], without string) *httptest.Server {
	t.Helper()
	const weightedType = "IReadOnlyList<IWeightedValue<String>>"
	properties, err := json.Marshal(map[string]any{"Products": map[string]any{
		ipi_cloud.DataKey: map[string]any{
			"DataTier": "CloudV5Bespoke",
			"Properties": []map[string]string{
				{"Name": name, "Type": weightedType, "Category": "Network"},
				{"Name": without, "Type": weightedType, "Category": "Network"},
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	written := make([]weightedValueJSON, 0, len(values))
	for _, v := range values {
		written = append(written, weightedValueJSON{v.Value(), v.RawWeighting()})
	}
	response, err := json.Marshal(map[string]any{ipi_cloud.DataKey: map[string]any{
		strings.ToLower(name):                   written,
		strings.ToLower(without):                nil,
		strings.ToLower(without) + "nullreason": cloudNoValueReason,
	}})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		w.Write(properties)
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["query.client-ip"]`))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Write(response)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// cloudData processes the address through a Pipeline of a Cloud Request
// Engine and the IP Intelligence Cloud Aspect Engine, against the stand-in
// cloud, and returns the Element Data held under the key.
func cloudData(t *testing.T, server *httptest.Server, address string) ipData {
	t.Helper()
	requestEngine, err := requestengine.NewBuilder().
		SetResourceKey("test-resource-key").
		SetEndpoint(server.URL + "/").
		SetHTTPClient(server.Client()).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := core.NewPipelineBuilder().
		AddFlowElement(requestEngine).
		AddFlowElement(ipi_cloud.New(nil)).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pipeline.Close() })
	return heldData(t, pipeline, map[string]any{"query.client-ip": address})
}

// heldData processes the Evidence and returns the Element Data the Flow Data
// holds under the key both Engines use, as the one interface.
func heldData(t *testing.T, pipeline core.Pipeline, evidence map[string]any) ipData {
	t.Helper()
	fd := pipeline.CreateFlowData()
	fd.AddEvidenceMap(evidence)
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	held, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("no Element Data under %q: %v", DataKey, err)
	}
	data, ok := held.(ipData)
	if !ok {
		t.Fatalf("the Element Data under %q is a %T, which is not Aspect "+
			"Data", DataKey, held)
	}
	if closer, ok := held.(interface{ Close() error }); ok {
		t.Cleanup(func() { closer.Close() })
	}
	return data
}

// readProperty reads a Property from either Engine's Element Data, by name,
// in the one way code written for both reads it.
func readProperty(data ipData, name string) (weightedText, error) {
	return core.GetPropertyAs[weightedText](data, name)
}

// The Element Data for a Cloud Aspect Engine MUST be interface compatible
// with the Element Data for the equivalent on-premise Engine. Both are held
// under the same key, both are Aspect Data, and the same Property is read
// from both by the same name, in any case, as the same Go type, with the
// same values and the same way of having no value.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/cloud-aspect-engine.md?plain=1#L24-L26
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L96-L98
func TestCloudAndOnPremiseElementDataAreReadInTheSameWay(t *testing.T) {
	if DataKey != ipi_cloud.DataKey {
		t.Fatalf("the on-premise key is %q and the cloud key is %q",
			DataKey, ipi_cloud.DataKey)
	}
	address := aspectTestAddresses[0]
	a := newTestAspectEngine(t)
	onPremisePipeline := newTestPipeline(t, a)
	query, _ := acceptedNames(t, a)
	onPremise := heldData(t, onPremisePipeline, map[string]any{query: address})

	// A Property the data file has a value of for the address, which the
	// stand-in cloud then gives the same values of.
	var name string
	var want []core.WeightedValue[string]
	for _, p := range a.Properties() {
		if !p.Available {
			continue
		}
		value, err := readProperty(onPremise, p.Name)
		if err != nil {
			t.Fatalf("reading %s on-premise: %v", p.Name, err)
		}
		if values, err := value.Value(); err == nil && len(values) > 0 {
			name, want = p.Name, values
			break
		}
	}
	if name == "" {
		t.Fatalf("no Property of %s has a value on-premise", address)
	}
	const without = "PropertyWithNoValue"
	cloud := cloudData(t, standInCloud(t, name, want, without), address)

	for _, written := range []string{name, strings.ToLower(name),
		strings.ToUpper(name)} {
		fromCloud, err := readProperty(cloud, written)
		if err != nil {
			t.Fatalf("reading %s from the cloud: %v", written, err)
		}
		fromFile, err := readProperty(onPremise, written)
		if err != nil {
			t.Fatalf("reading %s on-premise: %v", written, err)
		}
		cloudValues, err := fromCloud.Value()
		if err != nil {
			t.Fatal(err)
		}
		fileValues, err := fromFile.Value()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(cloudValues, fileValues) {
			t.Errorf("%s is %v from the cloud and %v on-premise", written,
				cloudValues, fileValues)
		}
	}

	// The untyped read gives the same Go type from both.
	cloudRaw, err := cloud.Get(name)
	if err != nil {
		t.Fatal(err)
	}
	fileRaw, err := onPremise.Get(name)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.TypeOf(cloudRaw) != reflect.TypeOf(fileRaw) {
		t.Errorf("%s is a %T from the cloud and a %T on-premise", name,
			cloudRaw, fileRaw)
	}
	if _, ok := cloud.AsMap()[strings.ToLower(name)].(weightedText); !ok {
		t.Errorf("the cloud's AsMap does not hold %s in lower case as "+
			"weighted text", name)
	}
	if _, ok := onPremise.AsMap()[strings.ToLower(name)].(weightedText); !ok {
		t.Errorf("the on-premise AsMap does not hold %s in lower case as "+
			"weighted text", name)
	}

	// A Property with no value is the same type from both, says it has no
	// value, and gives a NoValueError with a message when it is read.
	noAddress := heldData(t, onPremisePipeline, map[string]any{})
	for source, value := range map[string]func() (weightedText, error){
		"the cloud":  func() (weightedText, error) { return readProperty(cloud, without) },
		"on-premise": func() (weightedText, error) { return readProperty(noAddress, name) },
	} {
		got, err := value()
		if err != nil {
			t.Errorf("reading a Property with no value from %s: %v", source,
				err)
			continue
		}
		_, err = got.Value()
		var noValue *core.NoValueError
		if got.HasValue() || !errors.As(err, &noValue) || noValue.Message == "" {
			t.Errorf("a Property with no value from %s has a value %v or "+
				"gave %v, want a NoValueError with a message", source,
				got.HasValue(), err)
		}
	}

	// A name neither holds gives a PropertyMissingError from both.
	for source, data := range map[string]ipData{
		"the cloud": cloud, "on-premise": onPremise,
	} {
		_, err := data.Get("NotAPropertyOfEither")
		var missing *core.PropertyMissingError
		if !errors.As(err, &missing) {
			t.Errorf("a name %s does not hold gave %v, want a "+
				"PropertyMissingError", source, err)
		}
	}
}
