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

// With no Property metadata for the ip product, the engine still reads every
// value in the response's ip section, taking a list of values with weightings
// as weighted and anything else as text, and a reason member as the reason
// for its Property having no value, not as a Property.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/pipeline-elements/cloud-aspect-engine.md#start-up-activity
func TestValuesAreReadWithEmptyPropertyMetadata(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Products":{"ip":{"DataTier":"CloudV5Bespoke",` +
			`"Properties":[]}}}`))
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["query.client-ip"]`))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ip":{` +
			`"registeredcountry":[{"value":"GB","rawweighting":49151}],` +
			`"accuracyradiusmin":250,` +
			`"town":null,"townnullreason":"` + noTownReason + `"}}`))
	})
	srv := httptest.NewServer(mux)
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

	value, err := data.Get("registeredcountry")
	if err != nil {
		t.Fatalf("registeredcountry: %v", err)
	}
	countries, ok := value.(engines.AspectPropertyValue[[]core.WeightedValue[string]])
	if !ok {
		t.Fatalf("registeredcountry is a %T, want weighted values", value)
	}
	if list, err := countries.Value(); err != nil || len(list) != 1 ||
		list[0].Value() != "GB" || list[0].RawWeighting() != 49151 {
		t.Errorf("registeredcountry is %v, %v", list, err)
	}

	value, err = data.Get("accuracyradiusmin")
	if err != nil {
		t.Fatalf("accuracyradiusmin: %v", err)
	}
	radius, ok := value.(engines.AspectPropertyValue[string])
	if !ok {
		t.Fatalf("accuracyradiusmin is a %T, want text", value)
	}
	if text, err := radius.Value(); err != nil || text != "250" {
		t.Errorf("accuracyradiusmin is %q, %v", text, err)
	}

	value, err = data.Get("town")
	if err != nil {
		t.Fatalf("town: %v", err)
	}
	town, ok := value.(engines.AspectPropertyValue[string])
	if !ok {
		t.Fatalf("town is a %T, want text with no value", value)
	}
	if town.HasValue() || town.NoValueMessage() != noTownReason {
		t.Errorf("town has a value %v, or the reason %q",
			town.HasValue(), town.NoValueMessage())
	}

	if _, err := data.Get("townnullreason"); err == nil {
		t.Error("the reason for town is given as a Property of its own")
	}
}
