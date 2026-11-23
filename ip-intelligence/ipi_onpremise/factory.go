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
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// BuilderName is the name a configuration file gives the engine's builder,
// as the other languages' configuration files do. Registered under it,
// Factory also builds an element named IpiOnPremiseEngine.
const BuilderName = "IpiOnPremiseEngineBuilder"

// performanceProfiles are the performance profiles a configuration file can
// name, matched ignoring case.
var performanceProfiles = map[string]ipi_interop.PerformanceProfile{
	"default":         ipi_interop.Default,
	"lowmemory":       ipi_interop.LowMemory,
	"balancedtemp":    ipi_interop.BalancedTemp,
	"balanced":        ipi_interop.Balanced,
	"highperformance": ipi_interop.HighPerformance,
	"inmemory":        ipi_interop.InMemory,
	"maxperformance":  ipi_interop.InMemory,
}

// buildParameter is a build parameter the factory reads, by its names: the
// one the other languages' configuration files use, then the one the With
// option gives it in code.
type buildParameter struct {
	names  []string
	option func(params map[string]any, name string) (EngineOptions, error)
}

// buildParameters are the build parameters the factory reads, in the order
// their options are applied.
var buildParameters = []buildParameter{
	{[]string{"DataFile"}, stringOption(WithDataFile)},
	{[]string{"DataUpdateUrl"}, stringOption(WithDataUpdateUrl)},
	{[]string{"DataUpdateLicenseKey", "LicenseKey"}, stringOption(WithLicenseKey)},
	{[]string{"Product"}, stringOption(WithProduct)},
	{[]string{"AutoUpdate"}, boolOption(WithAutoUpdate)},
	{[]string{"DataFileSystemWatcher", "FileWatch"}, boolOption(WithFileWatch)},
	{[]string{"DataUpdateOnStartUp", "UpdateOnStart"}, boolOption(WithUpdateOnStart)},
	{[]string{"CreateTempDataCopy", "TempDataCopy"}, boolOption(WithTempDataCopy)},
	{[]string{"TempDirPath", "TempDataDir"}, stringOption(WithTempDataDir)},
	{[]string{"UpdatePollingInterval", "PollingInterval"}, intOption(WithPollingInterval)},
	{[]string{"UpdateRandomisationMax", "Randomization"}, intOption(WithRandomization)},
	{[]string{"MaxRetries"}, intOption(WithMaxRetries)},
	{[]string{"Logging"}, boolOption(WithLogging)},
	{[]string{"Properties"}, listOption(WithProperties)},
	{[]string{"PerformanceProfile"}, performanceProfileOption},
	{[]string{"Concurrency"}, concurrencyOption},
	{[]string{"ReuseOperationalFile"}, boolOption(WithReuseOperationalFile)},
}

// Factory builds the engine as a PipelineEngine from the build parameters of a
// configuration file, for core.BuilderRegistry, so a Pipeline holding it can
// be built from a configuration file. DataFile is needed. Each other
// parameter sets the option of the same name in code, and a parameter it
// does not know is refused, so a misspelt option is not passed over.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/pipeline-configuration.md?plain=1#L8-L10
func Factory(params map[string]any, logger *slog.Logger) (core.FlowElement, error) {
	opts, err := engineOptions(params)
	if err != nil {
		return nil, err
	}
	engine, err := New(append([]EngineOptions{withDefaultConcurrency()},
		opts...)...)
	if err != nil {
		return nil, err
	}
	return newPipelineEngine(engine, logger), nil
}

// engineOptions gives the options the build parameters set.
func engineOptions(params map[string]any) ([]EngineOptions, error) {
	if err := checkParameterNames(params); err != nil {
		return nil, err
	}
	if _, ok := core.StringParam(params, "DataFile"); !ok {
		return nil, errors.New("the build parameters name no DataFile, the " +
			"path of the data file the engine reads")
	}
	var opts []EngineOptions
	for _, p := range buildParameters {
		for _, name := range p.names {
			if _, ok := core.Param(params, name); !ok {
				continue
			}
			opt, err := p.option(params, name)
			if err != nil {
				return nil, err
			}
			opts = append(opts, opt)
			break
		}
	}
	return opts, nil
}

// checkParameterNames refuses a build parameter the factory does not read.
func checkParameterNames(params map[string]any) error {
	known := map[string]bool{}
	var names []string
	for _, p := range buildParameters {
		for _, name := range p.names {
			known[strings.ToLower(name)] = true
			names = append(names, name)
		}
	}
	var unknown []string
	for name := range params {
		if !known[strings.ToLower(name)] {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("the build parameters %s are not options of the "+
		"engine, which are %s", strings.Join(unknown, ", "),
		strings.Join(names, ", "))
}

// stringOption reads a build parameter as text for the option.
func stringOption(option func(string) EngineOptions) func(map[string]any, string) (EngineOptions, error) {
	return func(params map[string]any, name string) (EngineOptions, error) {
		v, ok := core.StringParam(params, name)
		if !ok {
			return nil, fmt.Errorf("the build parameter %s is not text", name)
		}
		return option(v), nil
	}
}

// boolOption reads a build parameter as true or false for the option.
func boolOption(option func(bool) EngineOptions) func(map[string]any, string) (EngineOptions, error) {
	return func(params map[string]any, name string) (EngineOptions, error) {
		v, ok := core.BoolParam(params, name)
		if !ok {
			return nil, fmt.Errorf("the build parameter %s is not true or false", name)
		}
		return option(v), nil
	}
}

// intOption reads a build parameter as a whole number for the option.
func intOption(option func(int) EngineOptions) func(map[string]any, string) (EngineOptions, error) {
	return func(params map[string]any, name string) (EngineOptions, error) {
		v, ok := core.IntParam(params, name)
		if !ok {
			return nil, fmt.Errorf("the build parameter %s is not a whole number", name)
		}
		return option(v), nil
	}
}

// listOption reads a build parameter as a comma separated list for the
// option.
func listOption(option func([]string) EngineOptions) func(map[string]any, string) (EngineOptions, error) {
	return func(params map[string]any, name string) (EngineOptions, error) {
		v, ok := core.StringListParam(params, name)
		if !ok {
			return nil, fmt.Errorf("the build parameter %s is not a list", name)
		}
		return option(v), nil
	}
}

// concurrencyOption reads the Concurrency build parameter, a whole number
// from 0 to 65535, as WithConcurrency.
func concurrencyOption(params map[string]any, name string) (EngineOptions, error) {
	v, ok := core.IntParam(params, name)
	if !ok || v < 0 || v > math.MaxUint16 {
		return nil, fmt.Errorf("the build parameter %s is not a whole "+
			"number from 0 to %d", name, math.MaxUint16)
	}
	return WithConcurrency(uint16(v)), nil
}

// performanceProfileOption reads the PerformanceProfile build parameter, the
// name of a performance profile, as WithPerformanceProfile.
func performanceProfileOption(params map[string]any, name string) (EngineOptions, error) {
	v, _ := core.StringParam(params, name)
	profile, ok := performanceProfiles[strings.ToLower(strings.TrimSpace(v))]
	if !ok {
		return nil, fmt.Errorf("the performance profile %q is not one of "+
			"Default, LowMemory, BalancedTemp, Balanced, HighPerformance, "+
			"InMemory or MaxPerformance", v)
	}
	return WithPerformanceProfile(profile), nil
}
