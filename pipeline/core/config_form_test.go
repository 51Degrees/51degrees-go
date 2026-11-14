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

package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The configuration file is as similar as possible to the other languages'
// files, which hold the options under a PipelineOptions member at the root.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/pipeline-configuration.md#flow-elements

// otherLanguagesForm is a file in the form of the .NET sample configuration,
// with a section beside PipelineOptions as an appsettings.json file has.
const otherLanguagesForm = `{
  "Logging": { "LogLevel": { "Default": "Information" } },
  "PipelineOptions": {
    "Elements": [
      { "BuilderName": "SequenceElement", "BuildParameters": {} },
      {
        "BuilderName": "ShareUsageBuilder",
        "BuildParameters": { "MinimumEntriesPerMessage": 1 }
      }
    ],
    "BuildParameters": { "SuppressProcessExceptions": true }
  }
}`

func writeConfigFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pipeline.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func checkOtherLanguagesForm(t *testing.T, opts PipelineOptions) {
	t.Helper()
	if len(opts.Elements) != 2 {
		t.Fatalf("read %d elements from a file with two under "+
			"PipelineOptions", len(opts.Elements))
	}
	if opts.Elements[0].BuilderName != "SequenceElement" ||
		opts.Elements[1].BuilderName != "ShareUsageBuilder" {
		t.Errorf("builder names %q and %q, want SequenceElement and "+
			"ShareUsageBuilder", opts.Elements[0].BuilderName,
			opts.Elements[1].BuilderName)
	}
	if v, _ := opts.Elements[1].BuildParameters["MinimumEntriesPerMessage"].(float64); v != 1 {
		t.Errorf("the second element's build parameters are %v",
			opts.Elements[1].BuildParameters)
	}
	if v, ok := boolParam(opts.BuildParameters, "suppressProcessExceptions"); !ok || !v {
		t.Errorf("the Pipeline's build parameters are %v", opts.BuildParameters)
	}
}

func TestAFileInTheOtherLanguagesFormLoadsItsElements(t *testing.T) {
	opts, err := LoadPipelineOptions(writeConfigFile(t, otherLanguagesForm))
	if err != nil {
		t.Fatal(err)
	}
	checkOtherLanguagesForm(t, opts)
}

// An application that reads the file itself and hands it to json.Unmarshal
// gets the same options.
func TestJSONUnmarshalReadsTheOtherLanguagesForm(t *testing.T) {
	var opts PipelineOptions
	if err := json.Unmarshal([]byte(otherLanguagesForm), &opts); err != nil {
		t.Fatal(err)
	}
	checkOtherLanguagesForm(t, opts)
}

// A file in the form earlier releases read, with the elements at the root,
// still loads.
func TestAFileWithTheElementsAtTheRootStillLoads(t *testing.T) {
	opts, err := LoadPipelineOptions(writeConfigFile(t,
		`{"elements": [{"builderName": "SequenceElement"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.Elements) != 1 ||
		opts.Elements[0].BuilderName != "SequenceElement" {
		t.Errorf("read %+v", opts.Elements)
	}
}

// A file that names no element would build a Pipeline that does nothing, so
// it is refused with a message that shows where the elements go.
func TestAFileThatNamesNoElementsIsRefused(t *testing.T) {
	for name, body := range map[string]string{
		"empty object":           `{}`,
		"empty PipelineOptions":  `{"PipelineOptions": {}}`,
		"empty Elements":         `{"PipelineOptions": {"Elements": []}}`,
		"another section only":   `{"Logging": {}}`,
		"empty elements at root": `{"elements": []}`,
		"misspelt wrapper":       `{"PipelineOption": {"Elements": [{"BuilderName": "A"}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := LoadPipelineOptions(writeConfigFile(t, body))
			var ce *ConfigurationError
			if !errors.As(err, &ce) {
				t.Fatalf("got %v, want a ConfigurationError", err)
			}
			if !strings.Contains(err.Error(), "names no elements") ||
				!strings.Contains(err.Error(), "PipelineOptions") {
				t.Errorf("the message %q does not say the file names no "+
					"elements and where they go", err.Error())
			}
		})
	}
}

// Elements both at the root and under PipelineOptions leave it unclear which
// were meant, so the file is refused.
func TestElementsInBothPlacesAreRefused(t *testing.T) {
	_, err := LoadPipelineOptions(writeConfigFile(t, `{
		"elements": [{"builderName": "A"}],
		"PipelineOptions": {"Elements": [{"BuilderName": "B"}]}}`))
	var ce *ConfigurationError
	if !errors.As(err, &ce) {
		t.Fatalf("got %v, want a ConfigurationError", err)
	}
}

func TestBuildFromConfigurationRefusesNoElements(t *testing.T) {
	_, err := NewBuilderRegistry().BuildFromConfiguration(
		PipelineOptions{}, nil)
	var ce *ConfigurationError
	if !errors.As(err, &ce) {
		t.Fatalf("got %v, want a ConfigurationError", err)
	}
}
