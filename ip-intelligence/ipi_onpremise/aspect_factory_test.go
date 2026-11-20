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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// writeConfiguration writes a configuration file that names the Engine with
// the build parameters, in the form every language reads, and returns its
// path.
func writeConfiguration(t *testing.T, builderName string, parameters map[string]any) string {
	t.Helper()
	file, err := json.MarshalIndent(map[string]any{
		"PipelineOptions": map[string]any{
			"Elements": []any{map[string]any{
				"BuilderName":     builderName,
				"BuildParameters": parameters,
			}},
		},
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "pipeline.json")
	if err := os.WriteFile(path, file, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildAspectFromFile builds a Pipeline from the configuration file, with AspectFactory
// registered under BuilderName.
func buildAspectFromFile(t *testing.T, path string) (core.Pipeline, error) {
	t.Helper()
	options, err := core.LoadPipelineOptions(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	registry := core.NewBuilderRegistry()
	registry.Register(BuilderName, AspectFactory)
	return registry.BuildFromConfiguration(options, nil)
}

// quietParameters are the build parameters of an Engine on the free Asn data
// file that downloads and watches nothing.
func quietParameters(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{
		ParamDataFile:   testDataPath(t, reloadTestDataFile),
		ParamAutoUpdate: false,
		ParamFileWatch:  false,
		ParamLogging:    false,
	}
}

// It is REQUIRED that elements and Pipelines can be configured and created
// using either a pure code approach, or a configuration file. A Pipeline
// holding the Engine is built from a file, with each option the
// specification names for this Engine, and gives the values the Engine
// created in code gives.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/pipeline-configuration.md?plain=1#L8-L10
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L611-L621
func TestAPipelineIsBuiltFromAConfigurationFile(t *testing.T) {
	inCode := newTestAspectEngine(t)
	var names []string
	for _, p := range inCode.Properties() {
		if p.Available {
			names = append(names, p.Name)
		}
	}
	if len(names) == 0 {
		t.Fatal("the Engine has no Properties")
	}
	for _, builderName := range []string{BuilderName, "IpiOnPremiseEngine"} {
		parameters := quietParameters(t)
		parameters[ParamPerformanceProfile] = "LowMemory"
		parameters[ParamConcurrency] = 2
		parameters[ParamReuseOperationalFile] = false
		parameters[ParamProperties] = strings.Join(names, ", ")
		pipeline, err := buildAspectFromFile(t,
			writeConfiguration(t, builderName, parameters))
		if err != nil {
			t.Fatalf("building from a file that names %s gave %v",
				builderName, err)
		}
		t.Cleanup(func() { pipeline.Close() })

		built, ok := pipeline.GetElement(DataKey).(*AspectEngine)
		if !ok {
			t.Fatalf("the Pipeline holds a %T under %q",
				pipeline.GetElement(DataKey), DataKey)
		}
		engine := built.Engine()
		if engine.config.PerformanceProfile() != ipi_interop.LowMemory {
			t.Errorf("the Performance Profile is %v, want LowMemory",
				engine.config.PerformanceProfile())
		}
		if engine.concurrency == nil || *engine.concurrency != 2 {
			t.Errorf("Concurrency is %v, want 2", engine.concurrency)
		}
		if len(built.names) != len(names) {
			t.Errorf("the Engine was created with %v, want %v", built.names,
				names)
		}

		query, _ := acceptedNames(t, built)
		for _, address := range aspectTestAddresses {
			data := processEvidence(t, pipeline,
				map[string]any{query: address})
			if checkValues(t, built, data, address) == 0 {
				t.Errorf("no Property of %s has a value", address)
			}
		}
	}
}

// A build parameter may be given as text, as a configuration file holds no
// type information, and its name is matched ignoring case.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/pipeline-configuration.md?plain=1#L50-L61
func TestBuildParametersMayBeText(t *testing.T) {
	element, err := AspectFactory(map[string]any{
		"datafile":             testDataPath(t, reloadTestDataFile),
		"AUTOUPDATE":           "false",
		"fileWatch":            "False",
		"logging":              "false",
		"performanceprofile":   "maxperformance",
		"concurrency":          "4",
		"reuseOperationalFile": "false",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer element.Close()
	engine := element.(*AspectEngine).Engine()
	if engine.config.PerformanceProfile() != ipi_interop.InMemory {
		t.Errorf("the Performance Profile is %v, want InMemory",
			engine.config.PerformanceProfile())
	}
	if engine.concurrency == nil || *engine.concurrency != 4 {
		t.Errorf("Concurrency is %v, want 4", engine.concurrency)
	}
	if engine.IsAutoUpdateEnabled() || engine.IsFileWatcherEnabled() {
		t.Error("AutoUpdate or FileWatch is on, and both were set to false")
	}
}

// badParameters are settings the build has to refuse, each with text its
// error has to hold.
var badParameters = []struct {
	name    string
	change  map[string]any
	message string
}{
	{"a parameter the Engine does not have",
		map[string]any{"DataFil": "data.ipi"}, "DataFil"},
	{"a Concurrency of zero",
		map[string]any{ParamConcurrency: 0}, "Concurrency"},
	{"a Concurrency below zero",
		map[string]any{ParamConcurrency: -4}, "Concurrency"},
	{"a Concurrency above the highest the native code takes",
		map[string]any{ParamConcurrency: 65536}, "Concurrency"},
	{"a Concurrency that is not a whole number",
		map[string]any{ParamConcurrency: 2.5}, "Concurrency"},
	{"a Concurrency that is not a number",
		map[string]any{ParamConcurrency: "many"}, "Concurrency"},
	{"a Performance Profile that does not exist",
		map[string]any{ParamPerformanceProfile: "Fastest"},
		"PerformanceProfile"},
	{"a Performance Profile that is not a name",
		map[string]any{ParamPerformanceProfile: 3}, "PerformanceProfile"},
	{"Reuse Operational File turned on",
		map[string]any{ParamReuseOperationalFile: true},
		"ReuseOperationalFile"},
	{"a switch that is neither true nor false",
		map[string]any{ParamAutoUpdate: "yes"}, "AutoUpdate"},
	{"a data file that is not text",
		map[string]any{ParamDataFile: 5}, "DataFile"},
	{"Properties that are not names",
		map[string]any{ParamProperties: 7}, "Properties"},
	{"a License Key with an address that takes none",
		map[string]any{ParamDataUpdateUrl: "https://example.com/data.ipi",
			ParamLicenseKey: "key"}, "license key"},
}

// A bad setting is refused when the element is built, by the builder, with
// an error that names the setting, and no element is returned.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L611-L621
func TestABadSettingIsRefusedWhenTheElementIsBuilt(t *testing.T) {
	for _, c := range badParameters {
		t.Run(c.name, func(t *testing.T) {
			parameters := quietParameters(t)
			for name, value := range c.change {
				parameters[name] = value
			}
			element, err := AspectFactory(parameters, nil)
			if err == nil {
				element.Close()
				t.Fatalf("the build took %v", c.change)
			}
			if element != nil {
				t.Errorf("the build gave a %T with the error %v", element,
					err)
			}
			if !strings.Contains(err.Error(), c.message) {
				t.Errorf("the error is %q, which does not name %s",
					err.Error(), c.message)
			}

			pipeline, err := buildAspectFromFile(t,
				writeConfiguration(t, BuilderName, parameters))
			if err == nil {
				pipeline.Close()
				t.Fatalf("a Pipeline was built from a file with %v",
					c.change)
			}
		})
	}
}

// A build with no data file is refused.
func TestABuildWithNoDataFileIsRefused(t *testing.T) {
	element, err := AspectFactory(map[string]any{ParamAutoUpdate: false}, nil)
	if err == nil {
		element.Close()
		t.Fatal("the build took no data file")
	}
	if element != nil {
		t.Errorf("the build gave a %T with the error %v", element, err)
	}
}

// A MUST binds every public way of getting the Engine, so the constructors
// that do not go through the builder refuse the same settings.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L611-L621
func TestABadSettingIsRefusedByEveryConstructor(t *testing.T) {
	path := testDataPath(t, reloadTestDataFile)
	for _, c := range []struct {
		name string
		opts []EngineOptions
	}{
		{"a Performance Profile that does not exist",
			[]EngineOptions{WithPerformanceProfile(
				ipi_interop.PerformanceProfile(99))}},
		{"Reuse Operational File turned on",
			[]EngineOptions{WithReuseOperationalFile(true)}},
		{"a Performance Profile and then a native configuration",
			[]EngineOptions{
				WithPerformanceProfile(ipi_interop.LowMemory),
				WithConfigIpi(ipi_interop.NewConfigIpi(ipi_interop.Balanced))}},
		{"a native configuration and then a Performance Profile",
			[]EngineOptions{
				WithConfigIpi(ipi_interop.NewConfigIpi(ipi_interop.Balanced)),
				WithPerformanceProfile(ipi_interop.LowMemory)}},
	} {
		t.Run(c.name, func(t *testing.T) {
			opts := append([]EngineOptions{
				WithDataFile(path),
				WithAutoUpdate(false),
				WithFileWatch(false),
				WithLogging(false),
			}, c.opts...)
			if engine, err := New(opts...); err == nil {
				engine.Stop()
				t.Error("New took the setting")
			}
			if engine, err := NewAspectEngine(opts...); err == nil {
				engine.Close()
				t.Error("NewAspectEngine took the setting")
			}
		})
	}
}

// The options set in code are applied as the same options set in a file
// are, and the native configuration given whole still takes a Concurrency.
func TestTheOptionsAreAppliedInCode(t *testing.T) {
	a := newTestAspectEngine(t,
		WithPerformanceProfile(ipi_interop.HighPerformance),
		WithConcurrency(2),
		WithReuseOperationalFile(false))
	if got := a.Engine().config.PerformanceProfile(); got != ipi_interop.HighPerformance {
		t.Errorf("the Performance Profile is %v, want HighPerformance", got)
	}
	if got := a.Engine().concurrency; got == nil || *got != 2 {
		t.Errorf("Concurrency is %v, want 2", got)
	}

	given := ipi_interop.NewConfigIpi(ipi_interop.LowMemory)
	b := newTestAspectEngine(t, WithConfigIpi(given), WithConcurrency(5))
	if b.Engine().config != given {
		t.Error("the Engine does not use the native configuration it was " +
			"given")
	}
}

// Each name the specification gives a Performance Profile is read, in any
// case, and any other name is refused with the names that can be used.
func TestParsePerformanceProfile(t *testing.T) {
	for name, want := range map[string]ipi_interop.PerformanceProfile{
		"LowMemory":       ipi_interop.LowMemory,
		"Balanced":        ipi_interop.Balanced,
		"BalancedTemp":    ipi_interop.BalancedTemp,
		"HighPerformance": ipi_interop.HighPerformance,
		"MaxPerformance":  ipi_interop.InMemory,
		"InMemory":        ipi_interop.InMemory,
		" lowmemory ":     ipi_interop.LowMemory,
	} {
		if got, err := ParsePerformanceProfile(name); err != nil || got != want {
			t.Errorf("%q is read as %v, %v, want %v", name, got, err, want)
		}
	}
	_, err := ParsePerformanceProfile("Fastest")
	if err == nil || !strings.Contains(err.Error(), "MaxPerformance") {
		t.Errorf("an unknown name gave %v, want an error that lists the "+
			"names", err)
	}
}
