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
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/cache"
)

// iPhoneEvidence is the evidence of an iPhone, for ProcessData.
var iPhoneEvidence = []Evidence{
	{Prefix: dd.HttpHeaderString, Key: "User-Agent", Value: aspectIPhone},
}

// closerOf gives data as an io.Closer, or fails t when it is not one.
func closerOf(t *testing.T, data any) io.Closer {
	t.Helper()
	closer, ok := data.(io.Closer)
	if !ok {
		t.Fatalf("%T has no Close, so the native results it reads from "+
			"cannot be released", data)
	}
	return closer
}

// The Element Data MUST read a Property value from ResultsHash only when that
// Property is requested, and MUST NOT copy Property values out of ResultsHash
// while the Engine processes the Flow Data.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L152-L157
func TestPipelineEngineCopiesNoValueWhileProcessing(t *testing.T) {
	data := onPremiseData(t)
	if stored := data.BaseAspectData.AsMap(); len(stored) != 0 {
		t.Errorf("processing stored %d values in the Element Data, want "+
			"none until a Property is requested", len(stored))
	}
	if got := aspectText(t, data, "PlatformName"); got != "iOS" {
		t.Errorf("PlatformName is %q, want iOS", got)
	}
}

// ProcessData, the engine used on its own, reads no value while it processes
// either, and its Data is released with Close.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L152-L157
func TestProcessDataCopiesNoValue(t *testing.T) {
	engine, err := New(WithDataFile(aspectDataFile), WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Stop()
	data, err := engine.ProcessData(iPhoneEvidence)
	if err != nil {
		t.Fatal(err)
	}
	defer closerOf(t, data).Close()
	if stored := data.BaseAspectData.AsMap(); len(stored) != 0 {
		t.Errorf("ProcessData stored %d values, want none until a Property "+
			"is requested", len(stored))
	}
}

// Close releases the native results at once, a read after it gives an error
// rather than reading released memory, and a second Close does nothing.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md?plain=1#L69-L70
func TestDeviceDataCloseReleasesTheResults(t *testing.T) {
	engine, err := New(WithDataFile(aspectDataFile), WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Stop()
	data, err := engine.ProcessData(iPhoneEvidence)
	if err != nil {
		t.Fatal(err)
	}
	closer := closerOf(t, data)
	if err := closer.Close(); err != nil {
		t.Fatalf("Close gave %v", err)
	}
	if _, err := data.Get("PlatformName"); err == nil {
		t.Error("a read after Close gave no error")
	}
	if err := closer.Close(); err != nil {
		t.Errorf("a second Close gave %v", err)
	}
}

// cacheSetter is an engine whose SetCache can refuse a cache.
type cacheSetter interface {
	SetCache(c cache.Cache) error
}

// The engine takes no results cache, because its Element Data reads from
// native results that are released, and a cached instance would be read
// after that.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md?plain=1#L83-L85
func TestPipelineEngineRefusesACache(t *testing.T) {
	engine, err := NewPipelineEngine(WithDataFile(aspectDataFile),
		WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	setter, ok := any(engine).(cacheSetter)
	if !ok {
		t.Fatal("SetCache gives no error, so a cache can be added to an " +
			"engine whose Data must be released")
	}
	err = setter.SetCache(cache.NewLRUCache(cache.DefaultSize, 0))
	if err == nil {
		t.Error("SetCache added a cache, want an error")
	} else if !strings.Contains(err.Error(), "cache") {
		t.Errorf("the error %q does not say the cache was refused", err)
	}
	if err := setter.SetCache(nil); err != nil {
		t.Errorf("SetCache(nil) gave %v, want nothing added and no error", err)
	}
}

// matchMetrics are the match metric Properties, in the order the engine
// lists them after the Properties of the data file.
var matchMetrics = []string{"MatchedNodes", "Difference", "Drift", "DeviceId",
	"UserAgents", "Iterations", "Method"}

// The engine's Property metadata lists the match metric Properties after the
// data file's, in the order of the specification's table, each in the
// "Device Metrics" category.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L448-L475
func TestPipelineEngineListsTheMatchMetricsLast(t *testing.T) {
	engine, err := NewPipelineEngine(WithDataFile(aspectDataFile),
		WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	properties := engine.Properties()
	if len(properties) <= len(matchMetrics) {
		t.Fatalf("the engine lists %d Properties, want the data file's and "+
			"the %d match metrics", len(properties), len(matchMetrics))
	}
	last := properties[len(properties)-len(matchMetrics):]
	for i, p := range last {
		if p.Name != matchMetrics[i] {
			t.Errorf("Property %d from the end is %s, want %s",
				len(matchMetrics)-i, p.Name, matchMetrics[i])
			continue
		}
		want := reflect.TypeOf("")
		if p.Name == "UserAgents" {
			want = reflect.TypeOf([]string(nil))
		}
		if p.Type != want {
			t.Errorf("%s is a %v, want %v", p.Name, p.Type, want)
		}
		if p.Category != "Device Metrics" || !p.Available ||
			p.Element != core.FlowElement(engine) || p.Description == "" {
			t.Errorf("%s has category %q, available %v, element %v and "+
				"description %q, want Device Metrics, available, this "+
				"engine and a description", p.Name, p.Category, p.Available,
				p.Element, p.Description)
		}
		if !reflect.DeepEqual(p.DataTiersWherePresent,
			[]string{"Lite", "Premium", "Enterprise", "TAC"}) {
			t.Errorf("%s is in the data files %v, want Lite, Premium, "+
				"Enterprise and TAC", p.Name, p.DataTiersWherePresent)
		}
	}
}

// The Element Data gives each match metric Property, as text where it is not
// a list, as the Device Detection Cloud Engine gives it.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L448-L475
func TestDeviceDataGivesTheMatchMetrics(t *testing.T) {
	data := onPremiseData(t)
	id := aspectText(t, data, "DeviceId")
	if !regexp.MustCompile(`^[0-9]+(-[0-9]+)*$`).MatchString(id) ||
		id == "0-0-0-0" {
		t.Errorf("DeviceId is %q, want the iPhone's profile ids joined by "+
			"hyphens", id)
	}
	method := aspectText(t, data, "Method")
	switch method {
	case "NONE", "PERFORMANCE", "COMBINED", "PREDICTIVE":
	default:
		t.Errorf("Method is %q, want one of NONE, PERFORMANCE, COMBINED "+
			"and PREDICTIVE", method)
	}
	for _, name := range []string{"MatchedNodes", "Difference", "Drift",
		"Iterations"} {
		text := aspectText(t, data, name)
		if n, err := strconv.Atoi(text); err != nil || n < 0 {
			t.Errorf("%s is %q, want a whole number of 0 or more", name, text)
		}
	}
	agents, err := data.Get("useragents")
	if err != nil {
		t.Fatalf("UserAgents is missing: %v", err)
	}
	if _, ok := agents.(engines.AspectPropertyValue[[]string]); !ok {
		t.Errorf("UserAgents is a %T, want engines.AspectPropertyValue"+
			"[[]string]", agents)
	}
	all := data.AsMap()
	for _, name := range matchMetrics {
		if _, ok := all[strings.ToLower(name)]; !ok {
			t.Errorf("AsMap has no %s", strings.ToLower(name))
		}
	}
}
