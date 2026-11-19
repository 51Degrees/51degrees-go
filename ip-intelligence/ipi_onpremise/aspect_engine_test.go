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
	"errors"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/missingproperty"
)

// aspectTestAddresses are looked up in the free Asn data file, which CI
// links into the module folder.
var aspectTestAddresses = []string{"1.1.1.1", "8.8.8.8"}

// weightedText is the type every Property of the Element Data is read as.
type weightedText = engines.AspectPropertyValue[[]core.WeightedValue[string]]

// The AspectEngine is a Flow Element and an Aspect Engine, and its Element
// Data is Aspect Data.
var (
	_ core.FlowElement     = (*AspectEngine)(nil)
	_ engines.AspectEngine = (*AspectEngine)(nil)
	_ engines.AspectData   = (*AspectData)(nil)
)

// newTestAspectEngine creates the AspectEngine on the free Asn data file,
// with nothing that downloads or watches, and closes it when the test ends.
func newTestAspectEngine(t *testing.T, opts ...EngineOptions) *AspectEngine {
	t.Helper()
	all := append([]EngineOptions{
		WithDataFile(testDataPath(t, reloadTestDataFile)),
		WithAutoUpdate(false),
		WithFileWatch(false),
		WithLogging(false),
	}, opts...)
	a, err := NewAspectEngine(all...)
	if err != nil {
		t.Fatalf("NewAspectEngine: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	return a
}

// newTestPipeline builds a Pipeline that holds only the AspectEngine, and
// closes it when the test ends.
func newTestPipeline(t *testing.T, a *AspectEngine) core.Pipeline {
	t.Helper()
	pipeline, err := core.NewPipelineBuilder().AddFlowElement(a).Build()
	if err != nil {
		t.Fatalf("building the Pipeline: %v", err)
	}
	t.Cleanup(func() { pipeline.Close() })
	return pipeline
}

// processEvidence processes the Evidence through the Pipeline and returns
// the AspectData, which it closes when the test ends.
func processEvidence(t *testing.T, pipeline core.Pipeline, evidence map[string]any) *AspectData {
	t.Helper()
	fd := pipeline.CreateFlowData()
	fd.AddEvidenceMap(evidence)
	if err := fd.Process(); err != nil {
		t.Fatalf("processing %v: %v", evidence, err)
	}
	data, err := core.Get[*AspectData](fd)
	if err != nil {
		t.Fatalf("no AspectData for %v: %v", evidence, err)
	}
	t.Cleanup(func() { data.Close() })
	return data
}

// readWeighted reads the Property through Get and fails the test when it is
// not weighted text.
func readWeighted(t *testing.T, data core.ElementData, name string) weightedText {
	t.Helper()
	raw, err := data.Get(name)
	if err != nil {
		t.Fatalf("Get(%s): %v", name, err)
	}
	value, ok := raw.(weightedText)
	if !ok {
		t.Fatalf("Get(%s) gave a %T, want a %T", name, raw, weightedText{})
	}
	return value
}

// checkValues fails the test unless the AspectData gives, for every Property
// the Engine has, the values the Engine's own Process gives for the address,
// each as text with its weight as a raw weighting. It returns how many
// Properties had a value.
func checkValues(t *testing.T, a *AspectEngine, data *AspectData, address string) int {
	t.Helper()
	want, err := a.Engine().Process(address)
	if err != nil {
		t.Fatalf("Process(%s): %v", address, err)
	}
	withValue := 0
	for _, name := range a.names {
		got := readWeighted(t, data, name)
		var expected []*ipi_interop.WeightedValue
		for wantName, values := range want {
			if strings.EqualFold(wantName, name) {
				expected = values
			}
		}
		if len(expected) == 0 {
			if got.HasValue() {
				t.Errorf("%s of %s has a value, and Process gave none",
					name, address)
			}
			continue
		}
		withValue++
		values, err := got.Value()
		if err != nil {
			t.Errorf("%s of %s: %v", name, address, err)
			continue
		}
		if len(values) != len(expected) {
			t.Errorf("%s of %s has %d values, want %d", name, address,
				len(values), len(expected))
			continue
		}
		for i, v := range values {
			if v.Value() != valueText(expected[i].Value) ||
				v.RawWeighting() != toRawWeighting(expected[i].Weight) {
				t.Errorf("%s of %s value %d is %q weighted %d, want %q "+
					"weighted %d", name, address, i, v.Value(),
					v.RawWeighting(), valueText(expected[i].Value),
					toRawWeighting(expected[i].Weight))
			}
		}
	}
	return withValue
}

// acceptedNames gives the first query key and the first server key the data
// file lists, as a Flow Data writes them.
func acceptedNames(t *testing.T, a *AspectEngine) (query, server string) {
	t.Helper()
	for _, key := range a.Engine().GetHttpHeaderKeys() {
		switch key.Prefix {
		case ipi_interop.HttpEvidenceQuery:
			if query == "" {
				query = evidenceKeyName(key)
			}
		case ipi_interop.HttpEvidenceServer:
			if server == "" {
				server = evidenceKeyName(key)
			}
		}
	}
	if query == "" || server == "" {
		t.Fatalf("the data file lists no query key or no server key: %v",
			a.Engine().GetHttpHeaderKeys())
	}
	return query, server
}

// All Engines MUST populate data objects that implement Aspect Data as
// defined in the Pipeline specification.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/data-model.md?plain=1#L10-L12
func TestTheEngineInAPipelinePopulatesAspectData(t *testing.T) {
	a := newTestAspectEngine(t)
	pipeline := newTestPipeline(t, a)
	if pipeline.GetElement(DataKey) != core.FlowElement(a) {
		t.Fatalf("the Pipeline does not hold the Engine under %q", DataKey)
	}
	query, _ := acceptedNames(t, a)

	fd := pipeline.CreateFlowData()
	fd.AddEvidence(query, aspectTestAddresses[0])
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	held, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("the Flow Data holds no Element Data under %q: %v",
			DataKey, err)
	}
	aspect, ok := held.(engines.AspectData)
	if !ok {
		t.Fatalf("the Element Data is a %T, which is not Aspect Data", held)
	}
	if aspect.CacheHit() {
		t.Error("the Aspect Data says it came from a cache, and the " +
			"Engine has none")
	}
	if closer, ok := held.(interface{ Close() error }); ok {
		closer.Close()
	}
}

// Element data returned by an IP Intelligence Engine MUST extend
// IAspectData. Each property MUST also be exposed as a weighted value.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/data-model.md?plain=1#L34-L37
func TestElementDataExtendsAspectDataAndEachPropertyIsWeighted(t *testing.T) {
	a := newTestAspectEngine(t)
	pipeline := newTestPipeline(t, a)
	query, _ := acceptedNames(t, a)
	if len(a.names) == 0 {
		t.Fatal("the Engine has no Properties")
	}
	for _, address := range aspectTestAddresses {
		data := processEvidence(t, pipeline, map[string]any{query: address})
		if data.BaseAspectData == nil {
			t.Fatal("the Element Data does not extend BaseAspectData")
		}
		if checkValues(t, a, data, address) == 0 {
			t.Errorf("no Property of %s has a value", address)
		}
		all := data.AsMap()
		for _, name := range a.names {
			if _, ok := all[name].(weightedText); !ok {
				t.Errorf("AsMap holds a %T for %s, want a %T", all[name],
					name, weightedText{})
			}
		}
	}
}

// A Property name is matched ignoring case, as the Cloud Aspect Engine's
// Element Data matches it.
func TestAPropertyIsReadByItsNameInAnyCase(t *testing.T) {
	a := newTestAspectEngine(t)
	query, _ := acceptedNames(t, a)
	data := processEvidence(t, newTestPipeline(t, a),
		map[string]any{query: aspectTestAddresses[0]})
	for _, p := range a.Properties() {
		if !p.Available {
			continue
		}
		for _, name := range []string{p.Name, strings.ToLower(p.Name),
			strings.ToUpper(p.Name)} {
			readWeighted(t, data, name)
		}
	}
}

// The Engine accepts the Evidence keys the data file lists, and no others,
// whatever the case of the key.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L73-L88
func TestTheEvidenceKeyFilterIsTheKeysTheDataFileLists(t *testing.T) {
	a := newTestAspectEngine(t)
	filter := a.EvidenceKeyFilter()
	keys := a.Engine().GetHttpHeaderKeys()
	if len(keys) == 0 {
		t.Fatal("the data file lists no Evidence keys")
	}
	for _, key := range keys {
		name := evidenceKeyName(key)
		for _, written := range []string{name, strings.ToUpper(name)} {
			if !filter.Include(written) {
				t.Errorf("the filter does not include %s", written)
			}
		}
	}
	for _, other := range []string{"header.user-agent", "client-ip",
		"cookie.client-ip", ""} {
		if filter.Include(other) {
			t.Errorf("the filter includes %q", other)
		}
	}
	pipeline := newTestPipeline(t, a)
	query, _ := acceptedNames(t, a)
	if !pipeline.EvidenceKeyFilter().Include(query) {
		t.Errorf("the Pipeline's filter does not include %s", query)
	}
}

// The native code chooses the address. A query key is used ahead of a server
// key, and a value that is not one whole address is passed over for the next
// key, without an error.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L150-L166
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L217-L222
func TestTheAddressIsChosenFromTheEvidence(t *testing.T) {
	a := newTestAspectEngine(t)
	pipeline := newTestPipeline(t, a)
	query, server := acceptedNames(t, a)
	first, second := aspectTestAddresses[0], aspectTestAddresses[1]
	for _, c := range []struct {
		name     string
		evidence map[string]any
		address  string
	}{
		{"the query key alone", map[string]any{query: first}, first},
		{"the server key alone", map[string]any{server: second}, second},
		{"the query key ahead of the server key",
			map[string]any{server: second, query: first}, first},
		{"a query value that is not an address",
			map[string]any{query: "unknown", server: second}, second},
		{"a query value that is a list of addresses",
			map[string]any{query: first + ", " + second, server: second},
			second},
		{"a key written in another case",
			map[string]any{strings.ToUpper(query): first}, first},
	} {
		t.Run(c.name, func(t *testing.T) {
			data := processEvidence(t, pipeline, c.evidence)
			if checkValues(t, a, data, c.address) == 0 {
				t.Errorf("no Property of %s has a value", c.address)
			}
		})
	}
}

// When no key holds a value that is read, processing raises no error and
// every Property has no value, with a message that says why. Reading the
// value gives an error.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L217-L222
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/properties.md?plain=1#L54-L58
func TestEvidenceWithNoAddressGivesNoValues(t *testing.T) {
	a := newTestAspectEngine(t)
	pipeline := newTestPipeline(t, a)
	query, _ := acceptedNames(t, a)
	for _, evidence := range []map[string]any{
		{},
		{query: "unknown"},
		{"header.user-agent": "a browser"},
	} {
		data := processEvidence(t, pipeline, evidence)
		for _, name := range a.names {
			value := readWeighted(t, data, name)
			if value.HasValue() {
				t.Errorf("%s has a value for the Evidence %v", name, evidence)
			}
			_, err := value.Value()
			var noValue *core.NoValueError
			if !errors.As(err, &noValue) || noValue.Message == "" {
				t.Errorf("reading %s gave %v, want a NoValueError with a "+
					"message", name, err)
			}
		}
	}
}

// A Property the data file defines that the Engine was not created with is
// missing because it was left out of the Engine's configuration, and a name
// the data file does not define is missing too. Neither is read as a value.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L400-L410
func TestAPropertyTheEngineWasNotCreatedWithIsMissing(t *testing.T) {
	all := newTestAspectEngine(t)
	if len(all.names) < 2 {
		t.Fatalf("the data file has %d Properties, and the test needs two",
			len(all.names))
	}
	var kept, left string
	for _, p := range all.Properties() {
		if kept == "" {
			kept = p.Name
		} else if left == "" {
			left = p.Name
		}
	}

	a := newTestAspectEngine(t, WithProperties([]string{kept}))
	query, _ := acceptedNames(t, a)
	data := processEvidence(t, newTestPipeline(t, a),
		map[string]any{query: aspectTestAddresses[0]})
	readWeighted(t, data, kept)

	_, err := data.Get(left)
	var missing *core.PropertyMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("Get(%s) gave %v, want a PropertyMissingError", left, err)
	}
	if missing.Reason != missingproperty.ReasonExcludedInConfig {
		t.Errorf("the reason %s is missing is %q, want %q", left,
			missing.Reason, missingproperty.ReasonExcludedInConfig)
	}
	if _, err := data.Get("NotAPropertyOfAnyDataFile"); !errors.As(err, &missing) {
		t.Errorf("a name the data file does not define gave %v, want a "+
			"PropertyMissingError", err)
	}
	for _, p := range a.Properties() {
		if p.Available != strings.EqualFold(p.Name, kept) {
			t.Errorf("the metadata says Available is %v for %s", p.Available,
				p.Name)
		}
	}
}

// The Engine exposes metadata for the Properties of the data file in use,
// and the Pipeline exposes it too.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/properties.md?plain=1#L67-L72
func TestTheEngineExposesThePropertyMetadataOfTheDataFile(t *testing.T) {
	a := newTestAspectEngine(t)
	metadata := a.Properties()
	want, err := a.Engine().Properties()
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) == 0 || len(metadata) != len(want) {
		t.Fatalf("the Engine has metadata for %d Properties, and the data "+
			"file defines %d", len(metadata), len(want))
	}
	for i, p := range metadata {
		if p.Name != want[i].Name || p.Category != want[i].Category ||
			p.Available != want[i].Available ||
			p.Element != core.FlowElement(a) || p.Type != valueType {
			t.Errorf("the metadata of %s is %+v, want that of %+v", p.Name,
				p, want[i])
		}
		if _, ok := a.positions[strings.ToLower(p.Name)]; ok != p.Available {
			t.Errorf("%s has Available %v, and a position is %v", p.Name,
				p.Available, ok)
		}
	}
	if got := newTestPipeline(t, a).Properties(); len(got) != len(metadata) {
		t.Errorf("the Pipeline has metadata for %d Properties, want %d",
			len(got), len(metadata))
	}
}

// Closing the Pipeline closes the Engine, which stops the Engine that holds
// the data file, and closing it again does nothing.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/resource-cleanup.md?plain=1#L22-L25
func TestClosingThePipelineStopsTheEngine(t *testing.T) {
	a, err := NewAspectEngine(
		WithDataFile(testDataPath(t, reloadTestDataFile)),
		WithAutoUpdate(false), WithFileWatch(false), WithLogging(false))
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := core.NewPipelineBuilder().AddFlowElement(a).Build()
	if err != nil {
		a.Close()
		t.Fatal(err)
	}
	if err := pipeline.Close(); err != nil {
		t.Fatal(err)
	}
	if !a.Engine().stopped() {
		t.Error("the Engine was not stopped when the Pipeline closed")
	}
	if err := a.Close(); err != nil {
		t.Errorf("closing the Engine again gave %v", err)
	}
}

// Concurrency is the number of processors unless it is set.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L620
func TestConcurrencyDefaultsToTheNumberOfProcessors(t *testing.T) {
	a := newTestAspectEngine(t)
	if got := a.Engine().defaultConcurrency; got != defaultConcurrency() || got == 0 {
		t.Errorf("the default Concurrency is %d, want %d", got,
			defaultConcurrency())
	}
	set := newTestAspectEngine(t, WithConcurrency(3))
	if got := set.Engine().concurrency; got == nil || *got != 3 {
		t.Errorf("Concurrency is %v, want 3", got)
	}
}
