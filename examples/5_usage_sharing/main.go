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

// Example 5: a Pipeline that shares usage with 51Degrees, configured from an
// options file.
//
// The Pipeline is built from config.json using a builder registry. For a
// self-contained, runnable example, a mock endpoint receives the shared data and
// the configured URL is overridden to point at it; in production the configured
// 51Degrees URL would be used unchanged.
//
// Run with: go run ./examples/5_usage_sharing
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/required-examples.md
package main

import (
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/elements/usagesharing"
	"github.com/51Degrees/pipeline-go/examples/starsign"
)

//go:embed config.json
var configJSON []byte

const evidenceDateOfBirth = "query.date-of-birth"

// starSignFactory builds the simple star sign element from configuration.
func starSignFactory(_ map[string]any, logger *slog.Logger) (core.FlowElement, error) {
	e := &simpleStarSign{BaseElement: core.NewBaseElement("starsign", core.NewWhitelistFilter(evidenceDateOfBirth), logger)}
	e.SetProperties([]core.PropertyMetadata{{Name: "starsign", Element: e, Available: true}})
	return e, nil
}

type simpleStarSign struct{ *core.BaseElement }

func (e *simpleStarSign) Process(fd core.FlowData) error {
	data := fd.GetOrAdd(e.DataKey(), func(core.FlowData) core.ElementData { return core.NewBaseElementData() })
	if raw, ok := fd.Evidence().Get(evidenceDateOfBirth); ok {
		if dob, err := starsign.ParseDate(fmt.Sprintf("%v", raw)); err == nil {
			data.Set("starsign", starsign.SignForDate(dob.Month(), dob.Day()))
			return nil
		}
	}
	data.Set("starsign", "Unknown")
	return nil
}

func main() {
	// Mock share-usage endpoint.
	var mu sync.Mutex
	received := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gz, err := gzip.NewReader(r.Body); err == nil {
			body, _ := io.ReadAll(gz)
			mu.Lock()
			received++
			mu.Unlock()
			fmt.Printf("Received usage payload (%d bytes of XML)\n", len(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Load configuration from the embedded options file.
	var opts core.PipelineOptions
	if err := json.Unmarshal(configJSON, &opts); err != nil {
		log.Fatal(err)
	}
	// Point usage sharing at the local mock endpoint for this example.
	for i := range opts.Elements {
		if opts.Elements[i].BuilderName == "ShareUsage" {
			opts.Elements[i].BuildParameters["ShareUsageUrl"] = srv.URL
		}
	}

	registry := core.NewBuilderRegistry()
	registry.Register("StarSign", starSignFactory)
	registry.Register("ShareUsage", usagesharing.Factory)

	pipeline, err := registry.BuildFromConfiguration(opts, nil)
	if err != nil {
		log.Fatal(err)
	}

	fd := pipeline.CreateFlowData()
	fd.AddEvidence(evidenceDateOfBirth, "1992-04-15")
	fd.AddEvidence("header.user-agent", "example-agent")
	if err := fd.Process(); err != nil {
		log.Fatal(err)
	}
	data, _ := fd.Get("starsign")
	sign, _ := data.Get("starsign")
	fmt.Printf("Star sign: %v\n", sign)

	// Closing the Pipeline flushes any queued usage data.
	if err := pipeline.Close(); err != nil {
		log.Fatal(err)
	}

	mu.Lock()
	fmt.Printf("Total usage payloads received: %d\n", received)
	mu.Unlock()
	_ = usagesharing.DataKey
}
