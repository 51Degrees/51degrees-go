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

package ipi_cloud

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// noTownReason is the reason the stand-in cloud gives for having no town.
const noTownReason = "The town is not known for this range."

// mockCloud stands in for the cloud service. It gives a weighted Property
// with two values, a Property that is not weighted, and a weighted Property
// with no value and a reason.
func mockCloud() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Products":{"ip":{"DataTier":"CloudV5Bespoke","Properties":[` +
			`{"Name":"RegisteredCountry","Type":"IReadOnlyList<IWeightedValue<String>>","Category":"Network"},` +
			`{"Name":"AccuracyRadiusMin","Type":"Int32","Category":"Location"},` +
			`{"Name":"Town","Type":"IReadOnlyList<IWeightedValue<String>>","Category":"Location"}]}}}`))
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["query.client-ip"]`))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ip":{` +
			`"registeredcountry":[{"value":"GB","rawweighting":49151},{"value":"IE","rawweighting":16384}],` +
			`"accuracyradiusmin":250,` +
			`"town":null,"townnullreason":"` + noTownReason + `"}}`))
	})
	return httptest.NewServer(mux)
}

// process runs one IP address through a Pipeline of a Cloud Request Engine
// and the IP Intelligence Cloud Aspect Engine, against the stand-in cloud.
func process(t *testing.T) *Data {
	srv := mockCloud()
	t.Cleanup(srv.Close)
	requestEngine, err := requestengine.NewBuilder().
		SetResourceKey("test-resource-key").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := core.NewPipelineBuilder().
		AddFlowElement(requestEngine).
		AddFlowElement(New(nil)).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pipeline.Close() })
	fd := pipeline.CreateFlowData()
	fd.AddEvidence("query.client-ip", "192.0.2.1")
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	data, err := core.Get[*Data](fd)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// A weighted Property keeps each value with the raw weighting the cloud gave.
func TestWeightedValue(t *testing.T) {
	value, err := process(t).Get("registeredcountry")
	if err != nil {
		t.Fatal(err)
	}
	countries, err := value.(engines.AspectPropertyValue[[]core.WeightedValue[string]]).Value()
	if err != nil {
		t.Fatal(err)
	}
	if len(countries) != 2 ||
		countries[0].Value() != "GB" || countries[0].RawWeighting() != 49151 ||
		countries[1].Value() != "IE" || countries[1].RawWeighting() != 16384 {
		t.Errorf("registeredcountry is %v", countries)
	}
}

// A Property that is not weighted is given as the text of its JSON value.
func TestValueThatIsNotWeighted(t *testing.T) {
	value, err := process(t).Get("accuracyradiusmin")
	if err != nil {
		t.Fatal(err)
	}
	radius, err := value.(engines.AspectPropertyValue[string]).Value()
	if err != nil || radius != "250" {
		t.Errorf("accuracyradiusmin is %q, %v", radius, err)
	}
}

// A Property with no value has none, and gives the reason the cloud gave.
func TestNoValue(t *testing.T) {
	value, err := process(t).Get("town")
	if err != nil {
		t.Fatal(err)
	}
	town := value.(engines.AspectPropertyValue[[]core.WeightedValue[string]])
	if town.HasValue() {
		t.Error("town has a value")
	}
	if town.NoValueMessage() != noTownReason {
		t.Errorf("the reason for no town is %q", town.NoValueMessage())
	}
}
