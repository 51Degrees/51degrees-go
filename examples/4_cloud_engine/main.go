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

// Example 4: a Cloud Engine that offloads star sign processing to a remote
// service.
//
// A Cloud Request Engine makes a single HTTP call per Flow Data, and a Cloud
// Aspect Engine deserializes the JSON response into typed Element Data. For a
// self-contained, runnable example, a mock 51Degrees cloud service is started
// locally; in production the engine would point at https://cloud.51degrees.com.
//
// Run with: go run ./examples/4_cloud_engine
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/required-examples.md
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/51Degrees/pipeline-go/cloud/aspectengine"
	"github.com/51Degrees/pipeline-go/cloud/requestengine"
	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/engines"
	"github.com/51Degrees/pipeline-go/examples/starsign"
)

const aspectKey = "starsign"

// CloudStarSignEngine deserializes the star sign Aspect from the cloud response.
type CloudStarSignEngine struct {
	*aspectengine.BaseCloudAspectEngine
}

// NewCloudStarSignEngine creates the engine.
func NewCloudStarSignEngine() *CloudStarSignEngine {
	e := &CloudStarSignEngine{BaseCloudAspectEngine: aspectengine.NewBaseCloudAspectEngine(aspectKey, nil)}
	e.SetProperties([]core.PropertyMetadata{
		{Name: "starsign", Element: e, Category: "Astrology", Available: true},
	})
	return e
}

// StarSignData is the typed Element Data produced from the cloud response.
type StarSignData struct {
	*engines.BaseAspectData
}

// Process parses the star sign Aspect from the cloud JSON.
func (e *CloudStarSignEngine) Process(fd core.FlowData) error {
	aspect, err := aspectengine.ParseAspect(fd, aspectKey)
	if err != nil {
		return err
	}
	data := &StarSignData{BaseAspectData: engines.NewBaseAspectData()}
	data.Set("starsign", aspectengine.PropertyValue[string](aspect, "starsign"))
	fd.GetOrAdd(aspectKey, func(core.FlowData) core.ElementData { return data })
	return nil
}

func main() {
	srv := mockCloud()
	defer srv.Close()

	requestEngine, err := requestengine.NewBuilder().
		SetResourceKey("example-resource-key").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	pipeline, err := core.NewPipelineBuilder().
		AddFlowElement(requestEngine).
		AddFlowElement(NewCloudStarSignEngine()).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	defer pipeline.Close()

	for _, dob := range []string{"1992-04-15", "1990-12-25"} {
		fd := pipeline.CreateFlowData()
		fd.AddEvidence("query.date-of-birth", dob)
		if err := fd.Process(); err != nil {
			log.Fatal(err)
		}
		data, _ := core.Get[*StarSignData](fd)
		apv, _ := data.Get("starsign")
		sign, _ := apv.(engines.AspectPropertyValue[string]).Value()
		fmt.Printf("%s -> %s (via cloud)\n", dob, sign)
	}
}

// mockCloud is a minimal stand-in for the 51Degrees cloud service.
func mockCloud() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Products":{"starsign":{"DataTier":"CloudV4","Properties":[{"Name":"starsign","Type":"String","Category":"Astrology"}]}}}`))
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["query.date-of-birth"]`))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		sign := "Unknown"
		if dob, err := starsign.ParseDate(r.PostForm.Get("date-of-birth")); err == nil {
			sign = starsign.SignForDate(dob.Month(), dob.Day())
		}
		resp := map[string]any{"starsign": map[string]any{"starsign": sign}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	return httptest.NewServer(mux)
}
