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
	"testing"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// aspectDataFile is the Lite data file the tests of the Aspect Data read.
const aspectDataFile = "../51Degrees-LiteV4.1.hash"

// aspectIPhone is the User-Agent of an iPhone, which the Lite data file
// finds to be a mobile running iOS.
const aspectIPhone = "Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) " +
	"AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 Safari/9537.53"

// aspectText gives the text of a Property that is not a list.
func aspectText(t *testing.T, data engines.AspectData, name string) string {
	t.Helper()
	value, err := data.Get(name)
	if err != nil {
		t.Fatalf("%s is missing: %v", name, err)
	}
	text, ok := value.(engines.AspectPropertyValue[string])
	if !ok {
		t.Fatalf("%s is a %T, want engines.AspectPropertyValue[string]", name, value)
	}
	got, err := text.Value()
	if err != nil {
		t.Fatalf("%s has no value: %v", name, err)
	}
	return got
}

// The engine used on its own, outside a Pipeline, gives its results as Aspect
// Data, as every way of getting them must.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/data-model.md?plain=1#L10-L12
func TestEngineGivesAspectData(t *testing.T) {
	engine, err := New(WithDataFile(aspectDataFile), WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Stop()
	data, err := engine.ProcessData([]Evidence{
		{Prefix: dd.HttpHeaderString, Key: "User-Agent", Value: aspectIPhone},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer data.Close()
	var aspect engines.AspectData = data
	if got := aspectText(t, aspect, "PlatformName"); got != "iOS" {
		t.Errorf("PlatformName is %q, want iOS", got)
	}
}

// The engine added to a Pipeline is an Aspect Engine, and adds its results to
// the Flow Data as Aspect Data under its Data Key.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/data-model.md?plain=1#L10-L12
func TestPipelineEngineAddsAspectData(t *testing.T) {
	engine, err := NewPipelineEngine(WithDataFile(aspectDataFile), WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	var _ engines.OnPremiseAspectEngine = engine
	pipeline, err := core.NewPipelineBuilder().AddFlowElement(engine).Build()
	if err != nil {
		engine.Close()
		t.Fatal(err)
	}
	defer pipeline.Close()
	fd := pipeline.CreateFlowData()
	fd.AddEvidence("header.user-agent", aspectIPhone)
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	data, err := core.Get[*DeviceData](fd)
	if err != nil {
		t.Fatal(err)
	}
	if got := aspectText(t, data, "platformname"); got != "iOS" {
		t.Errorf("platformname is %q, want iOS", got)
	}
	element, err := fd.Get(DataKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := element.(engines.AspectData); !ok {
		t.Errorf("the Element Data under %q is a %T, which is not Aspect Data",
			DataKey, element)
	}
}

// Evidence the engine does not accept is left out of its processing, and a
// Flow Data with none it accepts still gets Aspect Data, whose Properties
// have no values and say why.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/data-model.md?plain=1#L10-L12
func TestPipelineEngineWithNoEvidenceItAccepts(t *testing.T) {
	engine, err := NewPipelineEngine(WithDataFile(aspectDataFile), WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	if engine.EvidenceKeyFilter().Include("header.x-not-a-header-it-uses") {
		t.Error("the engine accepts a header it does not use")
	}
	if !engine.EvidenceKeyFilter().Include("header.user-agent") {
		t.Error("the engine does not accept header.user-agent")
	}
	pipeline, err := core.NewPipelineBuilder().AddFlowElement(engine).Build()
	if err != nil {
		engine.Close()
		t.Fatal(err)
	}
	defer pipeline.Close()
	fd := pipeline.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	data, err := core.Get[*DeviceData](fd)
	if err != nil {
		t.Fatal(err)
	}
	value, err := data.Get("ismobile")
	if err != nil {
		t.Fatal(err)
	}
	if accessor, ok := value.(engines.PropertyValueAccessor); !ok {
		t.Errorf("ismobile is a %T, not an Aspect Property Value", value)
	} else if !accessor.HasValue() && accessor.NoValueMessage() == "" {
		t.Error("ismobile has no value and no reason")
	}
}

// The engine gives the metadata of each Property it populates, naming itself
// as the element that populates it.
func TestPipelineEngineProperties(t *testing.T) {
	engine, err := NewPipelineEngine(WithDataFile(aspectDataFile), WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	found := false
	for _, p := range engine.Properties() {
		if p.Element != core.FlowElement(engine) {
			t.Errorf("%s names %v as its element", p.Name, p.Element)
		}
		if p.Name == "IsMobile" {
			found = p.Available
		}
	}
	if !found {
		t.Error("the engine's Properties lack an available IsMobile")
	}
}
