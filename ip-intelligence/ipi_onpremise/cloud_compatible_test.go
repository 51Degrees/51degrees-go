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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_cloud"
	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// compatibleAddress is the address the tests look up, which the ASN data
// file gives an AsnName for.
const compatibleAddress = reloadTestAddress

// compatibleDataFile gives the path of the free ASN data file, and skips the
// test outside CI when it has not been downloaded.
func compatibleDataFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", reloadTestDataFile)
	if _, err := os.Stat(path); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal(err)
		}
		t.Skipf("download %[1]s with cxx/data/get-data-files.sh %[1]s "+
			"(get-data-files.ps1 on Windows) and copy it from "+
			"cxx/data/ip-intelligence into the ip-intelligence folder to run "+
			"this test: %[2]v", reloadTestDataFile, err)
	}
	return path
}

// compatibleCloud stands in for the cloud service, giving AsnName as a
// weighted String, the type the cloud gives it as.
func compatibleCloud() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Products":{"ip":{"DataTier":"CloudV5Bespoke","Properties":[` +
			`{"Name":"AsnName","Type":"IReadOnlyList<IWeightedValue<String>>","Category":"Network"}]}}}`))
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["query.client-ip"]`))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ip":{"asnname":[{"value":"A network","rawweighting":65535}]}}`))
	})
	return httptest.NewServer(mux)
}

// compatibleCloudData gives the IP Intelligence Cloud Engine's Element Data
// from the stand-in cloud.
func compatibleCloudData(t *testing.T) *ipi_cloud.Data {
	srv := compatibleCloud()
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
		AddFlowElement(ipi_cloud.New(nil)).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pipeline.Close() })
	fd := pipeline.CreateFlowData()
	fd.AddEvidence("query.client-ip", compatibleAddress)
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	data, err := core.Get[*ipi_cloud.Data](fd)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// compatibleOnPremiseData gives the on-premise engine's Element Data, from a
// Pipeline given the address as the cloud is given it.
func compatibleOnPremiseData(t *testing.T) *IpData {
	engine, err := NewPipelineEngine(
		WithDataFile(compatibleDataFile(t)), WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	var _ engines.OnPremiseAspectEngine = engine
	pipeline, err := core.NewPipelineBuilder().AddFlowElement(engine).Build()
	if err != nil {
		engine.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { pipeline.Close() })
	fd := pipeline.CreateFlowData()
	fd.AddEvidence("query.client-ip", compatibleAddress)
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	data, err := core.Get[*IpData](fd)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The on-premise engine, in a Pipeline, gives Element Data under the IP
// Intelligence Cloud Engine's Data Key, and gives each Property the cloud
// gives as the same type, so code reading one reads the other.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/cloud-aspect-engine.md?plain=1#L24-L26
func TestElementDataIsCompatibleWithTheCloudEngines(t *testing.T) {
	if DataKey != ipi_cloud.DataKey {
		t.Errorf("the on-premise Data Key is %q and the cloud one %q",
			DataKey, ipi_cloud.DataKey)
	}
	onPremise := compatibleOnPremiseData(t)
	cloud := compatibleCloudData(t)
	want, err := cloud.Get("asnname")
	if err != nil {
		t.Fatal(err)
	}
	got, err := onPremise.Get("asnname")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%T", got) != fmt.Sprintf("%T", want) {
		t.Fatalf("asnname is a %T on-premise and a %T from the cloud", got, want)
	}
	values, err := got.(engines.AspectPropertyValue[[]core.WeightedValue[string]]).Value()
	if err != nil || len(values) == 0 || values[0].Value() == "" {
		t.Errorf("asnname for %s is %v, %v, want a name", compatibleAddress, values, err)
	}
}

// Used on its own, outside a Pipeline, the engine gives the same Element Data.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/cloud-aspect-engine.md?plain=1#L24-L26
func TestEngineGivesTheSameElementData(t *testing.T) {
	engine, err := New(WithDataFile(compatibleDataFile(t)), WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Stop()
	data, err := engine.ProcessData(compatibleAddress)
	if err != nil {
		t.Fatal(err)
	}
	var _ engines.AspectData = data
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
		_, weighted := value.(engines.AspectPropertyValue[[]core.WeightedValue[string]])
		_, text := value.(engines.AspectPropertyValue[string])
		if p.Type.IsWeighted() && !weighted || !p.Type.IsWeighted() && !text {
			t.Errorf("%s, weighted %v, is a %T", p.Name, p.Type.IsWeighted(), value)
		}
	}
}
