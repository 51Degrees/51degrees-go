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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// configurationFile writes a configuration file holding the engine, built by
// builderName with the build parameters given as JSON members, and gives its
// path.
func configurationFile(t *testing.T, builderName, parameters string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pipeline.json")
	text := `{"PipelineOptions": {"Elements": [{"BuilderName": "` + builderName +
		`", "BuildParameters": {` + parameters + `}}]}}`
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildFromFile builds a Pipeline from the configuration file, with the
// engine's factory registered under BuilderName.
func buildFromFile(t *testing.T, path string) (core.Pipeline, error) {
	t.Helper()
	options, err := core.LoadPipelineOptions(path)
	if err != nil {
		t.Fatal(err)
	}
	registry := core.NewBuilderRegistry()
	registry.Register(BuilderName, Factory)
	return registry.BuildFromConfiguration(options, nil)
}

// dataFileParameter gives the DataFile build parameter, naming the ASN data
// file as JSON.
func dataFileParameter(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(compatibleDataFile(t))
	if err != nil {
		t.Fatal(err)
	}
	return `"DataFile": "` + filepath.ToSlash(path) + `"`
}

// A Pipeline holding the engine is built from a configuration file, with the
// options the engine is configured with in code, under the names the other
// languages' configuration files use.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/pipeline-configuration.md?plain=1#L8-L10
func TestBuiltFromAConfigurationFile(t *testing.T) {
	for _, builderName := range []string{BuilderName, "IpiOnPremiseEngine"} {
		t.Run(builderName, func(t *testing.T) {
			path := configurationFile(t, builderName, dataFileParameter(t)+`,
				"AutoUpdate": false,
				"DataFileSystemWatcher": false,
				"DataUpdateOnStartUp": false,
				"CreateTempDataCopy": true,
				"PerformanceProfile": "LowMemory",
				"Properties": "AsnName"`)
			pipeline, err := buildFromFile(t, path)
			if err != nil {
				t.Fatalf("building from %s gave %v", path, err)
			}
			defer pipeline.Close()
			fd := pipeline.CreateFlowData()
			fd.AddEvidence("query.client-ip", compatibleAddress)
			if err := fd.Process(); err != nil {
				t.Fatal(err)
			}
			data, err := core.Get[*IpData](fd)
			if err != nil {
				t.Fatal(err)
			}
			value, err := data.Get("asnname")
			if err != nil {
				t.Fatal(err)
			}
			if !value.(engines.PropertyValueAccessor).HasValue() {
				t.Errorf("asnname for %s has no value", compatibleAddress)
			}
			if len(data.AsMap()) != 1 {
				t.Errorf("the data holds %v, although the Properties parameter "+
					"names only AsnName", data.AsMap())
			}
		})
	}
}

// The configuration file can name the options as the engine's With options
// do in code.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/pipeline-configuration.md?plain=1#L42-L43
func TestBuiltWithTheNamesUsedInCode(t *testing.T) {
	path := configurationFile(t, BuilderName, dataFileParameter(t)+`,
		"AutoUpdate": "false", "FileWatch": false, "UpdateOnStart": false,
		"TempDataCopy": false, "Properties": "AsnName"`)
	pipeline, err := buildFromFile(t, path)
	if err != nil {
		t.Fatalf("building from %s gave %v", path, err)
	}
	pipeline.Close()
}

// A configuration the engine cannot be built from is refused with an error
// that says why, and builds no engine.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/pipeline-configuration.md?plain=1#L8-L10
func TestARefusedConfiguration(t *testing.T) {
	for _, c := range []struct{ name, parameters, says string }{
		{"no data file", `"AutoUpdate": false`, "DataFile"},
		{"an unknown option", dataFileParameter(t) +
			`, "AutoUpdate": false, "DataFiel": "x"`, "DataFiel"},
		{"an unknown performance profile", dataFileParameter(t) +
			`, "AutoUpdate": false, "PerformanceProfile": "Fastest"`, "Fastest"},
	} {
		t.Run(c.name, func(t *testing.T) {
			pipeline, err := buildFromFile(t,
				configurationFile(t, BuilderName, c.parameters))
			if err == nil {
				pipeline.Close()
				t.Fatal("the Pipeline was built")
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("the error %q does not name %s", err, c.says)
			}
		})
	}
}
