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
	"math"
	"runtime"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// otherAddress is in a different range of the ASN data file from
// compatibleAddress.
const otherAddress = "8.8.8.8"

// processorCount is the Concurrency the engine uses when none is set.
func processorCount() uint16 {
	if n := runtime.NumCPU(); n < math.MaxUint16 {
		return uint16(n)
	}
	return math.MaxUint16
}

// newTestPipelineEngine creates the Pipeline engine on the ASN data file,
// with no update, and closes it when the test ends.
func newTestPipelineEngine(t *testing.T, opts ...EngineOptions) *PipelineEngine {
	t.Helper()
	p, err := NewPipelineEngine(testEngineOptions(t, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

// testEngineOptions puts the ASN data file and no update before opts.
func testEngineOptions(t *testing.T, opts ...EngineOptions) []EngineOptions {
	return append([]EngineOptions{
		WithDataFile(compatibleDataFile(t)), WithAutoUpdate(false)}, opts...)
}

// concurrencyOf gives the Concurrency the engine's data set was created with.
func concurrencyOf(p *PipelineEngine) uint16 {
	return p.Engine().config.Concurrency()
}

// The options table gives Concurrency the default "System processor count".
// A Concurrency set in code is used, and so is the one a native
// configuration given whole already holds.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L620
func TestConcurrencyIsTheProcessorCountUnlessSet(t *testing.T) {
	if got := concurrencyOf(newTestPipelineEngine(t)); got != processorCount() {
		t.Errorf("Concurrency is %d by default, want the processor count %d",
			got, processorCount())
	}
	if got := concurrencyOf(newTestPipelineEngine(t,
		WithConcurrency(3))); got != 3 {
		t.Errorf("WithConcurrency(3) gave a Concurrency of %d", got)
	}
	given := ipi_interop.NewConfigIpi(ipi_interop.Balanced)
	given.SetConcurrency(5)
	if got := concurrencyOf(newTestPipelineEngine(t,
		WithConfigIpi(given))); got != 5 {
		t.Errorf("a ConfigIpi holding 5 gave a Concurrency of %d", got)
	}
	if got := concurrencyOf(newTestPipelineEngine(t,
		WithConfigIpi(ipi_interop.NewConfigIpi(ipi_interop.Balanced)),
		WithConcurrency(2))); got != 2 {
		t.Errorf("WithConcurrency(2) with a ConfigIpi gave a Concurrency of %d",
			got)
	}
}

// WithPerformanceProfile sets the Performance Profile, and the default
// Concurrency still applies. A value that is not a Performance Profile is
// refused.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L619-L620
func TestWithPerformanceProfileSetsTheProfile(t *testing.T) {
	p := newTestPipelineEngine(t, WithPerformanceProfile(ipi_interop.LowMemory))
	if got := p.Engine().config.PerformanceProfile(); got != ipi_interop.LowMemory {
		t.Errorf("the Performance Profile is %v, want LowMemory", got)
	}
	if got := concurrencyOf(p); got != processorCount() {
		t.Errorf("Concurrency is %d with a profile, want the processor count %d",
			got, processorCount())
	}
	bad, err := NewPipelineEngine(testEngineOptions(t,
		WithPerformanceProfile(ipi_interop.PerformanceProfile(99)))...)
	if err == nil {
		bad.Close()
		t.Error("Performance Profile 99 was not refused")
	}
}

// WithPerformanceProfile and WithConfigIpi each set the Performance Profile,
// so giving both is refused, in either order.
func TestAPerformanceProfileGivenTwoWaysIsRefused(t *testing.T) {
	config := func() EngineOptions {
		return WithConfigIpi(ipi_interop.NewConfigIpi(ipi_interop.Balanced))
	}
	profile := WithPerformanceProfile(ipi_interop.LowMemory)
	for name, opts := range map[string][]EngineOptions{
		"profile first": {profile, config()},
		"config first":  {config(), profile},
	} {
		p, err := NewPipelineEngine(testEngineOptions(t, opts...)...)
		if err == nil {
			p.Close()
			t.Errorf("%s: both were accepted", name)
			continue
		}
		if !strings.Contains(err.Error(), "WithPerformanceProfile") {
			t.Errorf("%s: the error %q does not name WithPerformanceProfile",
				name, err)
		}
	}
}

// The factory reads Concurrency, which a configuration file can give with
// the Performance Profile, and refuses a number that cannot be one. With no
// Concurrency the processor count is used.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/pipeline-configuration.md?plain=1#L8-L10
func TestTheFactoryReadsConcurrency(t *testing.T) {
	build := func(extra map[string]any) (*PipelineEngine, error) {
		params := map[string]any{
			"DataFile": compatibleDataFile(t), "AutoUpdate": false}
		for k, v := range extra {
			params[k] = v
		}
		element, err := Factory(params, nil)
		if err != nil {
			return nil, err
		}
		p := element.(*PipelineEngine)
		t.Cleanup(func() { p.Close() })
		return p, nil
	}
	p, err := build(map[string]any{
		"PerformanceProfile": "LowMemory", "Concurrency": 2})
	if err != nil {
		t.Fatal(err)
	}
	if got := concurrencyOf(p); got != 2 {
		t.Errorf("Concurrency 2 gave %d", got)
	}
	if got := p.Engine().config.PerformanceProfile(); got != ipi_interop.LowMemory {
		t.Errorf("the Performance Profile is %v, want LowMemory", got)
	}
	if p, err = build(nil); err != nil {
		t.Fatal(err)
	} else if got := concurrencyOf(p); got != processorCount() {
		t.Errorf("Concurrency is %d by default, want the processor count %d",
			got, processorCount())
	}
	for _, bad := range []int{-1, math.MaxUint16 + 1} {
		if _, err := build(map[string]any{"Concurrency": bad}); err == nil {
			t.Errorf("Concurrency %d was not refused", bad)
		} else if !strings.Contains(err.Error(), "Concurrency") {
			t.Errorf("Concurrency %d gave %q, which does not name it", bad, err)
		}
	}
}

// firstKeys gives the first query key and the first server key the data file
// lists, as a Flow Data holds them.
func firstKeys(t *testing.T, e *Engine) (query, server string) {
	t.Helper()
	for _, k := range e.GetHttpHeaderKeys() {
		key := strings.ToLower(k.Key)
		switch {
		case k.Prefix == ipi_interop.HttpEvidenceQuery && query == "":
			query = "query." + key
		case k.Prefix == ipi_interop.HttpEvidenceServer && server == "":
			server = "server." + key
		}
	}
	if query == "" || server == "" {
		t.Fatalf("the data file lists no query key or no server key: %v",
			e.GetHttpHeaderKeys())
	}
	return query, server
}

// rangeStartValues gives the IpRangeStart values in values.
func rangeStartValues(values ipi_interop.Values) []*ipi_interop.WeightedValue {
	for name, list := range values {
		if strings.EqualFold(name, "IpRangeStart") {
			return list
		}
	}
	return nil
}

// The native code chooses the IP address from the evidence. A query key is
// used ahead of a server key, and a value that is not one whole address is
// passed over for the next key.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L114-L121
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L150-L175
func TestTheNativeCodeChoosesTheAddress(t *testing.T) {
	p := newTestPipelineEngine(t)
	pipeline, err := core.NewPipelineBuilder().AddFlowElement(p).Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pipeline.Close() })
	query, server := firstKeys(t, p.Engine())
	first, second := compatibleAddress, otherAddress
	for _, c := range []struct {
		name     string
		evidence map[string]string
		address  string
	}{
		{"the query key alone", map[string]string{query: first}, first},
		{"the server key alone", map[string]string{server: second}, second},
		{"the query key ahead of the server key",
			map[string]string{server: second, query: first}, first},
		{"a query value that is not an address",
			map[string]string{query: "unknown", server: second}, second},
		{"a query value that is a list of addresses",
			map[string]string{query: first + ", " + second, server: second},
			second},
	} {
		t.Run(c.name, func(t *testing.T) {
			fd := pipeline.CreateFlowData()
			defer fd.Close()
			for k, v := range c.evidence {
				fd.AddEvidence(k, v)
			}
			if err := fd.Process(); err != nil {
				t.Fatalf("processing gave %v", err)
			}
			data, err := core.Get[*Data](fd)
			if err != nil {
				t.Fatal(err)
			}
			want := rangeStartValues(processValues(t, p.Engine(), c.address))
			if len(want) == 0 {
				t.Fatalf("%s has no IpRangeStart", c.address)
			}
			checkText(t, data, "IpRangeStart", want)
		})
	}
}
