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
	"encoding/json"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// A Pipeline holding the engine can be built from a configuration file, in
// the form every language reads, by registering Factory beside the Cloud
// Request Engine's.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/pipeline-configuration.md?plain=1#L8-L10
func TestBuiltFromConfiguration(t *testing.T) {
	srv := mockCloud()
	defer srv.Close()
	registry := core.NewBuilderRegistry()
	registry.Register("CloudRequestEngine", requestengine.Factory)
	registry.Register("IpiCloudEngine", Factory)
	file := `{"PipelineOptions": {"Elements": [
		{"BuilderName": "CloudRequestEngine", "BuildParameters": {
			"ResourceKey": "test-resource-key", "EndPoint": "` + srv.URL + `/"}},
		{"BuilderName": "IpiCloudEngine"}]}}`
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
		t.Fatal(err)
	}
	countries, err := value.(engines.AspectPropertyValue[[]core.WeightedValue[string]]).Value()
	if err != nil || len(countries) != 2 ||
		countries[0].Value() != "GB" || countries[1].Value() != "IE" {
		t.Errorf("registeredcountry is %v, %v, want GB then IE", countries, err)
	}
}
