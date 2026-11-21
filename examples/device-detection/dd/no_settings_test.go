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

package dd

import (
	"html"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// settings holds each environment variable the examples read.
var settings = []string{"_51DEGREES_DD_PATH", "_51DEGREES_EVIDENCE_YAML"}

// withoutSettings returns the environment without each variable the examples
// read, as in a fresh clone.
func withoutSettings(environment []string) []string {
	read := map[string]bool{}
	for _, name := range settings {
		read[name] = true
	}
	kept := make([]string, 0, len(environment))
	for _, variable := range environment {
		name := strings.SplitN(variable, "=", 2)[0]
		if !read[strings.ToUpper(name)] {
			kept = append(kept, variable)
		}
	}
	return kept
}

// TestWithoutSettingsDropsEveryNameTheExamplesRead checks each variable the
// examples read is left out, and others are kept.
func TestWithoutSettingsDropsEveryNameTheExamplesRead(t *testing.T) {
	environment := []string{"PATH=kept"}
	for _, name := range settings {
		environment = append(environment, name+"=x")
	}
	got := withoutSettings(environment)
	if len(got) != 1 || got[0] != "PATH=kept" {
		t.Errorf("withoutSettings kept %q, want only PATH", got)
	}
}

// Each example runs with no arguments or settings, as from a fresh clone, so
// it uses the free data file and shows the link to ask for the Enterprise one.
// The other tests run the examples with the data file CI names.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/device-detection-specification/required-examples.md?plain=1#L20-L22
func TestEachExampleRunsWithNoSettings(t *testing.T) {
	environment := withoutSettings(os.Environ())
	for _, name := range examples {
		command := exec.Command("go", "run", "./"+name)
		command.Env = environment
		output, err := command.CombinedOutput()
		if err != nil {
			t.Errorf("%s did not run with no settings: %v\n%s", name, err,
				output)
			continue
		}
		// The file is named first, so a failure below says which file the
		// example loaded.
		if !strings.Contains(string(output), DataFileLine) {
			t.Errorf("%s ran with no settings and did not say which data "+
				"file it used:\n%s", name, output)
		}
		if !strings.Contains(html.UnescapeString(string(output)),
			contactUsLink) {
			t.Errorf("%s ran with no settings and gave no link to ask for "+
				"the Enterprise data file, so it did not use the free "+
				"one:\n%s", name, output)
		}
		if err := CheckFindOutMore(string(output)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
