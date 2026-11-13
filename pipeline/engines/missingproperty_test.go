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

package engines

import (
	"errors"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/cache"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/missingproperty"
)

// These tests follow the missing Properties section of the Pipeline
// specification, which says reading a Property an Engine defines and the
// result does not hold gives an error explaining why.
// See https://github.com/51Degrees/specifications/blob/a0a7008a6875b1b8aae4f947acd5252466c777fe/pipeline-specification/features/properties.md#missing-properties

// starData is Aspect Data with a typed accessor for StarSign.
type starData struct{ *BaseAspectData }

func (d *starData) StarSign() (any, error) { return d.Get("starsign") }

// starEngine is an On-premise Engine on the Lite tier. Its metadata lists
// Planet, which the Lite tier does not hold, and Element, which is excluded.
// Its Element Data holds neither, nor StarSign, which has a typed accessor.
type starEngine struct{ *BaseAspectEngine }

// starEngineType is this implementation's own name for the starEngine.
const starEngineType = "engines.starEngine"

func newStarEngine() *starEngine {
	e := &starEngine{NewBaseAspectEngine("star",
		core.NewWhitelistFilter("query.date"), nil)}
	e.SetDataSourceTier("Lite")
	e.SetProperties([]core.PropertyMetadata{
		{Name: "Planet", Available: true,
			DataTiersWherePresent: []string{"Premium", "Enterprise"}},
		{Name: "Element", DataTiersWherePresent: []string{"Lite"}},
	})
	return e
}

func (e *starEngine) Process(fd core.FlowData) error {
	return e.Execute(fd, func() (AspectData, error) {
		return &starData{BaseAspectData: NewBaseAspectData()}, nil
	})
}

// starPipeline builds a Pipeline holding e.
func starPipeline(t *testing.T, e *starEngine) core.Pipeline {
	t.Helper()
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build the Pipeline: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

// starResult processes one Flow Data through p and gives the star data.
func starResult(t *testing.T, p core.Pipeline) core.ElementData {
	t.Helper()
	fd := p.CreateFlowData()
	fd.AddEvidence("query.date", "1 January")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, err := fd.Get("star")
	if err != nil {
		t.Fatalf("no star data: %v", err)
	}
	return data
}

// checkMissing reads name from data and compares the error with want, which
// one of the missingproperty builders gives.
func checkMissing(t *testing.T, data core.ElementData, name string,
	want *core.PropertyMissingError) {
	t.Helper()
	_, err := data.Get(name)
	var got *core.PropertyMissingError
	if !errors.As(err, &got) {
		t.Fatalf("reading %s gives %v, want a PropertyMissingError", name, err)
	}
	if got.Reason != want.Reason || got.Error() != want.Error() {
		t.Errorf("reading %s\n got: %s, %s\nwant: %s, %s", name,
			got.Reason, got.Error(), want.Reason, want.Error())
	}
}

// checkStarReasons reads each missing Property of the star data.
func checkStarReasons(t *testing.T, data core.ElementData) {
	t.Helper()
	checkMissing(t, data, "planet", missingproperty.ForDataFileUpgrade(
		"planet", "star", starEngineType, []string{"Premium", "Enterprise"}))
	checkMissing(t, data, "ELEMENT",
		missingproperty.ForExcludedInConfig("ELEMENT", "star"))
	checkMissing(t, data, "starsign", missingproperty.ForDataFileUpgrade(
		"starsign", "star", starEngineType, nil))
	checkMissing(t, data, "Moon", missingproperty.Generic("Moon", "star"))
}

// Aspect Data an Engine adds to a Flow Data explains each missing Property
// from that Engine, in the order the specification gives.
func TestAspectDataExplainsAMissingProperty(t *testing.T) {
	checkStarReasons(t, starResult(t, starPipeline(t, newStarEngine())))
}

// The typed accessor gives the same error as reading the Property by name.
func TestTypedAccessorExplainsAMissingProperty(t *testing.T) {
	data := starResult(t, starPipeline(t, newStarEngine())).(*starData)
	_, err := data.StarSign()
	want := missingproperty.ForDataFileUpgrade("starsign", "star",
		starEngineType, nil)
	if err == nil || err.Error() != want.Error() {
		t.Errorf("StarSign gives %v, want %v", err, want)
	}
}

// Aspect Data served from the cache explains a missing Property as the
// Aspect Data first computed does.
func TestCachedAspectDataExplainsAMissingProperty(t *testing.T) {
	e := newStarEngine()
	e.SetCache(cache.NewLRUCache(100, 4))
	p := starPipeline(t, e)
	checkStarReasons(t, starResult(t, p))
	cached := starResult(t, p)
	if !cached.(AspectData).CacheHit() {
		t.Fatal("the second Flow Data was not served from the cache")
	}
	checkStarReasons(t, cached)
}

// Lazily loaded Aspect Data explains a missing Property too.
func TestLazyAspectDataExplainsAMissingProperty(t *testing.T) {
	e := newStarEngine()
	e.SetLazyLoading(true, time.Second)
	checkStarReasons(t, starResult(t, starPipeline(t, e)))
}

// Aspect Data that no Flow Data holds has no Engine to explain a missing
// Property from, so it gives the error any Element Data gives.
func TestAspectDataWithNoEngineGivesTheElementDataError(t *testing.T) {
	_, err := NewBaseAspectData().Get("planet")
	_, want := core.NewBaseElementData().Get("planet")
	if err == nil || err.Error() != want.Error() {
		t.Errorf("reading planet gives %v, want %v", err, want)
	}
}
