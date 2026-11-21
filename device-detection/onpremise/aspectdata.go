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
	"errors"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// DataKey is the key of the engine's Element Data in a Flow Data, the same as
// the Device Detection Cloud Engine's.
const DataKey = "device"

// ErrDataFreed is returned by a read from a DeviceData after its Close or
// Free.
var ErrDataFreed = errors.New("the Device Detection data has been released")

// deviceResults is the part of dd.ResultsHash that DeviceData reads.
type deviceResults interface {
	RequiredPropertyIndexFromName(name string) int
	AvailableProperties() []string
	HasValuesByIndex(index int) (bool, error)
	NoValueReasonMessageByIndex(index int) (string, error)
	Values(index int) ([]string, error)
	MatchedNodes() int32
	Difference() int32
	Drift() int32
	Iterations() int32
	DeviceId() (string, error)
	UserAgents() uint32
	UserAgent(index int) (string, error)
	Method() dd.MatchMethod
	Free()
}

// DeviceData is the Aspect Data the engine gives, from ProcessData or as a
// Pipeline engine. It holds the native results, and reads a Property's values
// from them only when that Property is requested, keeping what it has read
// so a second request does not call the native code again. Processing reads
// no value.
//
// Each Property is given under its name in any case, and AsMap gives it in
// lower case. A Property the data file holds as a list gives an
// engines.AspectPropertyValue[[]string], and any other Property an
// engines.AspectPropertyValue[string]. A Property with no value says why.
// The match metric Properties follow the same forms, UserAgents being a list
// and the others text.
//
// DeviceData is not safe to use from several goroutines at once, because
// reading a value writes into the native results and into the values
// DeviceData keeps, and DeviceData takes no lock. The caller is responsible
// for thread-safe access to one it shares, for example with a sync.Mutex of
// its own.
//
// The native results hold memory the Go garbage collector does not manage or
// see, so call Close, or Free, when the DeviceData is no longer needed, which
// releases them at once. A read after that returns ErrDataFreed. Closing the
// Flow Data that holds a DeviceData closes it. A DeviceData the Pipeline
// engine adds to a Flow Data also has a finalizer that releases them once
// nothing holds the DeviceData, as a backstop for a caller that closes
// neither.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/data-model.md?plain=1#L10-L12
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L152-L160
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L327-L332
type DeviceData struct {
	*engines.BaseAspectData
	results deviceResults
	indexOf func(name string) int
	kinds   map[string]propertyKind
	read    map[string]any
}

// ProcessData detects the device from the evidence, as Process does, and
// gives the results as DeviceData, which reads each Property's values from
// the native results only when that Property is requested. The caller calls
// Close, or Free, on the DeviceData when it is done with it. The DeviceData
// is not safe to use from several goroutines at once, so it belongs to one
// goroutine at a time. The caller is responsible for thread-safe access to
// one it shares.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/data-model.md?plain=1#L10-L12
func (e *Engine) ProcessData(evidenceList []Evidence) (*DeviceData, error) {
	results, err := e.Process(evidenceList)
	if err != nil {
		return nil, err
	}
	return e.newDeviceData(results), nil
}

// newDeviceData wraps results in a DeviceData, reading no value.
func (e *Engine) newDeviceData(results deviceResults) *DeviceData {
	return &DeviceData{
		BaseAspectData: engines.NewBaseAspectData(),
		results:        results,
		indexOf:        results.RequiredPropertyIndexFromName,
		kinds:          e.propertyKinds(),
		read:           make(map[string]any),
	}
}

// Get returns the value of a Property by its name, in any case. A Property of
// the data file, or a match metric Property, is read from the native results
// when it is requested, and kept. Any other name is looked up in the values
// Set has stored, and gives an error when it is not there. A Property of the
// engine read after Close or Free gives ErrDataFreed.
func (d *DeviceData) Get(propertyName string) (any, error) {
	key := strings.ToLower(propertyName)
	if value, ok := d.read[key]; ok {
		return value, nil
	}
	index := d.indexOf(propertyName)
	if index < 0 && !isMatchMetric(key) {
		return d.BaseAspectData.Get(propertyName)
	}
	if d.results == nil {
		return nil, ErrDataFreed
	}
	var value any
	if index >= 0 {
		value = d.propertyValue(key, index)
	} else {
		value = d.metricValue(key)
	}
	d.read[key] = value
	return value, nil
}

// AsMap returns the values Set has stored with every Property of the data
// file the engine was made with and every match metric Property, each under
// its name in lower case as Get gives it, reading from the native results
// any not yet read. After Close or Free, those not yet read are left out.
func (d *DeviceData) AsMap() map[string]any {
	m := d.BaseAspectData.AsMap()
	if m == nil {
		m = make(map[string]any)
	}
	if d.results != nil {
		for _, name := range d.results.AvailableProperties() {
			d.Get(name)
		}
		for _, name := range matchMetricNames {
			d.Get(name)
		}
	}
	for key, value := range d.read {
		m[key] = value
	}
	return m
}

// Free releases the native results. It is safe to call more than once.
func (d *DeviceData) Free() {
	if d.results != nil {
		d.results.Free()
		d.results = nil
	}
}

// Close releases the native results at once, as Free does, so that
// DeviceData is an io.Closer, and drops the finalizer a Pipeline sets. It is
// safe to call more than once, and after Free, which is what that finalizer
// calls. It always returns nil.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md?plain=1#L69-L70
func (d *DeviceData) Close() error {
	d.Free()
	runtime.SetFinalizer(d, nil)
	return nil
}

// propertyValue reads the Property of the data file at the required property
// index, whose name in lower case is key, in the form Get gives it.
func (d *DeviceData) propertyValue(key string, index int) any {
	values, reason := propertyValues(d.results, index)
	kind := d.kinds[key]
	switch {
	case kind == listProperty && reason == "":
		return engines.NewAspectPropertyValue(values)
	case kind == listProperty:
		return engines.NoValue[[]string](reason)
	case reason == "" && kind == booleanProperty:
		return engines.NewAspectPropertyValue(
			strings.ToLower(strings.Join(values, "|")))
	case reason == "":
		return engines.NewAspectPropertyValue(strings.Join(values, "|"))
	default:
		return engines.NoValue[string](reason)
	}
}

// propertyKind is how DeviceData gives the value of a Property.
type propertyKind int

const (
	// textProperty is given as its text, as the data file holds it.
	textProperty propertyKind = iota
	// listProperty is given as a list of Strings.
	listProperty
	// booleanProperty is given as "true" or "false", as the Device Detection
	// Cloud Engine gives a Boolean, where the data file holds "True" or
	// "False".
	booleanProperty
)

// propertyValues gives the values of the Property at the required property
// index, or the reason it has none.
func propertyValues(results deviceResults, index int) ([]string, string) {
	has, err := results.HasValuesByIndex(index)
	if err != nil {
		return nil, err.Error()
	}
	if !has {
		reason, err := results.NoValueReasonMessageByIndex(index)
		if err != nil {
			return nil, err.Error()
		}
		return nil, noValueReason(reason)
	}
	values, err := results.Values(index)
	if err != nil {
		return nil, err.Error()
	}
	if len(values) == 0 {
		return nil, noValueReason("")
	}
	return values, ""
}

// noValueReason gives a reason for a Property with no value, where the data
// file gave none.
func noValueReason(reason string) string {
	if reason == "" {
		return "The data file holds no value of this Property for the evidence."
	}
	return reason
}

// propertyKinds gives, by lower case name, how DeviceData gives each Property
// of the data file in use that is not text. The answer is kept once the data
// file has given it.
func (e *Engine) propertyKinds() map[string]propertyKind {
	if kinds := e.kinds.Load(); kinds != nil {
		return *kinds
	}
	properties, err := e.Properties()
	if err != nil {
		return map[string]propertyKind{}
	}
	kinds := make(map[string]propertyKind, len(properties))
	for _, p := range properties {
		switch {
		case p.IsList:
			kinds[strings.ToLower(p.Name)] = listProperty
		case p.Type == reflect.TypeOf(true):
			kinds[strings.ToLower(p.Name)] = booleanProperty
		}
	}
	e.kinds.Store(&kinds)
	return kinds
}

// matchMetric describes a match metric Property, which the native results
// give about the match rather than from the data file.
type matchMetric struct {
	name        string
	list        bool
	description string
}

// metricProperties are the match metric Properties, in the order of the
// specification's table, which is the order the engine lists them in after
// the Properties of the data file.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L448-L475
var metricProperties = []matchMetric{
	{"MatchedNodes", false, "Indicates the number of hash nodes matched " +
		"within the Evidence."},
	{"Difference", false, "Used when detection method is not Exact or " +
		"None. The larger the value the less confident the detector is in " +
		"this result."},
	{"Drift", false, "The drift of every component's result added " +
		"together, each result's drift being how far, in character " +
		"positions, the substring's hashes were found from where they were " +
		"expected. Never the largest or smallest result alone."},
	{"DeviceId", false, "Contains the profile ids of the matching " +
		"profiles, separated by a hyphen symbol. For example " +
		"[HardwareId]-[PlatformId]-[BrowserId]-[CrawlerId]. By convention, " +
		"these will be in component Id order. There will often be 4 ids " +
		"present, but this is not guaranteed."},
	{"UserAgents", true, "The User-Agent values that were matched, each " +
		"listed once across every result. Populated when the Update " +
		"Matched User Agent configuration option is on, which it is by " +
		"default."},
	{"Iterations", false, "The number of iterations carried out in order " +
		"to find a match. This is the number of nodes in the graph which " +
		"have been visited."},
	{"Method", false, "The method used to determine the match result."},
}

// matchMetricCategory and matchMetricTiers are the category of every match
// metric Property and the data files that give them.
const matchMetricCategory = "Device Metrics"

var matchMetricTiers = []string{"Lite", "Premium", "Enterprise", "TAC"}

// matchMetricNames are the names of the match metric Properties, in order.
var matchMetricNames = func() []string {
	names := make([]string, len(metricProperties))
	for i, m := range metricProperties {
		names[i] = m.name
	}
	return names
}()

// isMatchMetric reports whether key, a name in lower case, names a match
// metric Property.
func isMatchMetric(key string) bool {
	for _, name := range matchMetricNames {
		if strings.ToLower(name) == key {
			return true
		}
	}
	return false
}

// methodNames are the values of the Method Property.
var methodNames = map[dd.MatchMethod]string{
	dd.None:        "NONE",
	dd.Performance: "PERFORMANCE",
	dd.Combined:    "COMBINED",
	dd.Predictive:  "PREDICTIVE",
}

// metricValue reads the match metric Property whose name in lower case is
// key, in the form Get gives it.
func (d *DeviceData) metricValue(key string) any {
	number := func(n int32) any {
		return engines.NewAspectPropertyValue(strconv.Itoa(int(n)))
	}
	switch key {
	case "matchednodes":
		return number(d.results.MatchedNodes())
	case "difference":
		return number(d.results.Difference())
	case "drift":
		return number(d.results.Drift())
	case "iterations":
		return number(d.results.Iterations())
	case "deviceid":
		id, err := d.results.DeviceId()
		if err != nil {
			return engines.NoValue[string](err.Error())
		}
		return engines.NewAspectPropertyValue(id)
	case "useragents":
		return d.userAgents()
	default:
		return engines.NewAspectPropertyValue(methodNames[d.results.Method()])
	}
}

// userAgents gives the User-Agents the results matched, each listed once.
func (d *DeviceData) userAgents() any {
	agents := []string{}
	seen := map[string]bool{}
	for i := 0; i < int(d.results.UserAgents()); i++ {
		agent, err := d.results.UserAgent(i)
		if err != nil {
			return engines.NoValue[[]string](err.Error())
		}
		if agent != "" && !seen[agent] {
			seen[agent] = true
			agents = append(agents, agent)
		}
	}
	return engines.NewAspectPropertyValue(agents)
}
