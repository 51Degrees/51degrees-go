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
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// The names of the build parameters Factory reads from a configuration
// file. A name is matched ignoring case. Performance Profile, Concurrency
// and Reuse Operational File are the options of this Engine that the
// specification names, and each other parameter has the name of the option
// that sets it in code, without its With.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L611-L621
const (
	// ParamDataFile is the path of the data file, which is required.
	ParamDataFile = "DataFile"
	// ParamProperties lists the Properties the Engine is created with, as
	// names with commas between them. Left out, the Engine has every
	// Property of the data file.
	ParamProperties = "Properties"
	// ParamPerformanceProfile is the name of the Performance Profile, one of
	// LowMemory, Balanced, BalancedTemp, HighPerformance and MaxPerformance.
	ParamPerformanceProfile = "PerformanceProfile"
	// ParamConcurrency is the expected number of operations that use the
	// Engine at the same time, from 1 to 65535.
	ParamConcurrency = "Concurrency"
	// ParamReuseOperationalFile can only be false, as
	// WithReuseOperationalFile explains.
	ParamReuseOperationalFile = "ReuseOperationalFile"
	// ParamAutoUpdate turns the download of a newer data file on or off.
	ParamAutoUpdate = "AutoUpdate"
	// ParamFileWatch turns watching the data file for a change on or off.
	ParamFileWatch = "FileWatch"
	// ParamUpdateOnStart turns the download of a newer data file when the
	// Engine is created on or off.
	ParamUpdateOnStart = "UpdateOnStart"
	// ParamPollingInterval is the number of seconds between checks for a
	// newer data file.
	ParamPollingInterval = "PollingInterval"
	// ParamRandomization is the most seconds added at random to each
	// interval between checks.
	ParamRandomization = "Randomization"
	// ParamMaxRetries is the number of times a failed download is tried
	// again.
	ParamMaxRetries = "MaxRetries"
	// ParamDataUpdateUrl is the address a newer data file is downloaded
	// from.
	ParamDataUpdateUrl = "DataUpdateUrl"
	// ParamLicenseKey is the License Key a download from 51Degrees needs.
	ParamLicenseKey = "LicenseKey"
	// ParamProduct is the product a download from 51Degrees asks for.
	ParamProduct = "Product"
	// ParamTempDataCopy says whether the Engine works from a copy of the
	// data file.
	ParamTempDataCopy = "TempDataCopy"
	// ParamTempDataDir is the folder the copy of the data file is made in.
	ParamTempDataDir = "TempDataDir"
	// ParamLogging turns the Engine's own log messages on or off.
	ParamLogging = "Logging"
)

// BuilderName is the name to register Factory under. A configuration file
// names the Engine by it, with or without the Builder at its end, as the
// other languages' configuration files do.
const BuilderName = "IpiOnPremiseEngineBuilder"

// parameter reads one build parameter from the value a configuration file
// gave it, and returns the option that sets it.
type parameter struct {
	name string
	read func(value any) (EngineOptions, error)
}

// parameters are the build parameters Factory reads, in the order their
// options are applied. The data update address comes before the License Key
// and the product, so that a License Key given with an address that takes
// none is refused.
var parameters = []parameter{
	{ParamDataFile, textParameter(WithDataFile)},
	{ParamProperties, listParameter(WithProperties)},
	{ParamPerformanceProfile, profileParameter},
	{ParamConcurrency, numberParameter(WithConcurrency)},
	{ParamReuseOperationalFile, switchParameter(WithReuseOperationalFile)},
	{ParamAutoUpdate, switchParameter(WithAutoUpdate)},
	{ParamFileWatch, switchParameter(WithFileWatch)},
	{ParamUpdateOnStart, switchParameter(WithUpdateOnStart)},
	{ParamPollingInterval, numberParameter(WithPollingInterval)},
	{ParamRandomization, numberParameter(WithRandomization)},
	{ParamMaxRetries, numberParameter(WithMaxRetries)},
	{ParamDataUpdateUrl, textParameter(WithDataUpdateUrl)},
	{ParamLicenseKey, textParameter(WithLicenseKey)},
	{ParamProduct, textParameter(WithProduct)},
	{ParamTempDataCopy, switchParameter(WithTempDataCopy)},
	{ParamTempDataDir, textParameter(WithTempDataDir)},
	{ParamLogging, switchParameter(WithLogging)},
}

// Factory builds the AspectEngine from configuration for
// core.BuilderRegistry, so a Pipeline holding it can be built from a
// configuration file. The build parameters are the Param constants. A
// parameter it does not know, a value of the wrong type and a setting the
// Engine refuses each stop the build with an error that names the parameter.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/pipeline-configuration.md?plain=1#L8-L10
func Factory(params map[string]any, logger *slog.Logger) (core.FlowElement, error) {
	opts, err := optionsFrom(params)
	if err != nil {
		return nil, err
	}
	// A nil element is returned on failure, not a nil *AspectEngine, which
	// would make a core.FlowElement that is not nil.
	engine, err := newAspectEngine(logger, opts)
	if err != nil {
		return nil, core.NewConfigurationError(
			"the IP Intelligence On-premise Engine could not be built", err)
	}
	return engine, nil
}

// optionsFrom gives the options for the build parameters, in the order of
// parameters, and refuses a parameter that is not one of them.
func optionsFrom(params map[string]any) ([]EngineOptions, error) {
	given := make(map[string]any, len(params))
	for name, value := range params {
		lower := strings.ToLower(name)
		if _, twice := given[lower]; twice {
			return nil, core.NewConfigurationError(fmt.Sprintf(
				"the build parameter '%s' is given more than once, in "+
					"different cases. Give it once", name), nil)
		}
		given[lower] = value
	}
	var opts []EngineOptions
	for _, p := range parameters {
		value, ok := given[strings.ToLower(p.name)]
		if !ok {
			continue
		}
		delete(given, strings.ToLower(p.name))
		opt, err := p.read(value)
		if err != nil {
			return nil, core.NewConfigurationError(fmt.Sprintf(
				"the build parameter '%s' cannot be used", p.name), err)
		}
		opts = append(opts, opt)
	}
	if len(given) > 0 {
		unknown := make([]string, 0, len(given))
		for name := range given {
			unknown = append(unknown, name)
		}
		sort.Strings(unknown)
		known := make([]string, 0, len(parameters))
		for _, p := range parameters {
			known = append(known, p.name)
		}
		return nil, core.NewConfigurationError(fmt.Sprintf(
			"the IP Intelligence On-premise Engine has no build parameter "+
				"named '%s'. Its build parameters are %s",
			strings.Join(unknown, "', '"), strings.Join(known, ", ")), nil)
	}
	return opts, nil
}

// textParameter reads a parameter that is text.
func textParameter(option func(string) EngineOptions) func(any) (EngineOptions, error) {
	return func(value any) (EngineOptions, error) {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("its value is %v, and it has to be text",
				value)
		}
		return option(text), nil
	}
}

// listParameter reads a parameter that is a list of names, given as text
// with commas between the names or as a list.
func listParameter(option func([]string) EngineOptions) func(any) (EngineOptions, error) {
	return func(value any) (EngineOptions, error) {
		var parts []string
		switch v := value.(type) {
		case string:
			parts = strings.Split(v, ",")
		case []string:
			parts = v
		case []any:
			for _, item := range v {
				text, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("it holds %v, and each entry "+
						"has to be text", item)
				}
				parts = append(parts, text)
			}
		default:
			return nil, fmt.Errorf("its value is %v, and it has to be "+
				"names with commas between them", value)
		}
		names := make([]string, 0, len(parts))
		for _, part := range parts {
			if name := strings.TrimSpace(part); name != "" {
				names = append(names, name)
			}
		}
		return option(names), nil
	}
}

// switchParameter reads a parameter that is true or false, given as a
// boolean or as the text true or false.
func switchParameter(option func(bool) EngineOptions) func(any) (EngineOptions, error) {
	return func(value any) (EngineOptions, error) {
		switch v := value.(type) {
		case bool:
			return option(v), nil
		case string:
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "true":
				return option(true), nil
			case "false":
				return option(false), nil
			}
		}
		return nil, fmt.Errorf("its value is %v, and it has to be true or "+
			"false", value)
	}
}

// numberParameter reads a parameter that is a whole number, given as a
// number or as text.
func numberParameter(option func(int) EngineOptions) func(any) (EngineOptions, error) {
	return func(value any) (EngineOptions, error) {
		switch v := value.(type) {
		case int:
			return option(v), nil
		case int64:
			if v >= math.MinInt32 && v <= math.MaxInt32 {
				return option(int(v)), nil
			}
		case float64:
			// A configuration file's numbers are read as float64.
			if v == math.Trunc(v) && v >= math.MinInt32 && v <= math.MaxInt32 {
				return option(int(v)), nil
			}
		case string:
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				return option(n), nil
			}
		}
		return nil, fmt.Errorf("its value is %v, and it has to be a whole "+
			"number", value)
	}
}

// profileParameter reads the Performance Profile, given by its name.
func profileParameter(value any) (EngineOptions, error) {
	name, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("its value is %v, and it has to be the name "+
			"of a Performance Profile. Use %s", value, performanceProfileList)
	}
	profile, err := ParsePerformanceProfile(name)
	if err != nil {
		return nil, err
	}
	return WithPerformanceProfile(profile), nil
}
