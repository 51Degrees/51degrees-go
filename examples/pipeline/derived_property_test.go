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

package examples

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The headings the Derived Property example prints before each Pipeline.
const (
	fromConfiguration = "1. Built from config.json"
	fromCode          = "2. Built in code"
)

// The lines the example prints for the second request of each Pipeline,
// where session.PointerMoved is absent. The message names the missing
// source Property.
const (
	engagementMissing = "derived.ReaderEngagement: no value. " +
		"Derived property 'ReaderEngagement' has no value because 1 " +
		"source property was not available. 'session.PointerMoved'"
	everyTestMissing = "derived.ReaderEngagementIsHigh: no value. " +
		"Derived property 'ReaderEngagementIsHigh' has no value because " +
		"1 source property was not available. 'session.PointerMoved'"
)

// TestDerivedPropertyExample runs the Derived Property example as a fresh
// clone would, from another folder, and checks that it shows the four things
// pipeline-specification/required-examples.md asks of it: a script held as
// text, a script read from the file beside the example, the same Pipeline
// built from a configuration file, and a Property with no value and a message
// naming the missing source Property.
func TestDerivedPropertyExample(t *testing.T) {
	example, err := filepath.Abs("7_derived_property")
	if err != nil {
		t.Fatal(err)
	}
	// Start the built program from a folder that holds neither the script
	// nor the configuration file, so that the example finds both itself.
	output, err := runFrom(t, example, t.TempDir())
	os.Stdout.Write(output)
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	configured, coded, ok := sections(text)
	if !ok {
		t.Fatalf("the output does not hold %q followed by %q",
			fromConfiguration, fromCode)
	}
	for _, test := range []struct {
		section string
		where   string
		want    []string
	}{
		{configured, "the Pipeline built from config.json", []string{
			"ReaderEngagement.yaml",
			"derived.ReaderEngagement: High",
			"derived.ReaderEngagement: Medium",
			engagementMissing,
		}},
		{coded, "the Pipeline built in code", []string{
			"derived.ReaderEngagement: High",
			"derived.ReaderEngagementIsHigh: true",
			"derived.ReaderEngagementIsHigh: false",
			engagementMissing,
			everyTestMissing,
		}},
	} {
		for _, want := range test.want {
			if !strings.Contains(test.section, want) {
				t.Errorf("%s does not show %q", test.where, want)
			}
		}
	}
	if strings.Contains(configured, "ReaderEngagementIsHigh") {
		t.Error("the Pipeline built from config.json shows the script " +
			"held as text, which only the Pipeline built in code holds")
	}
}

// runFrom builds the example in folder into a temporary folder and runs it
// with dir as its working directory.
func runFrom(t *testing.T, folder, dir string) ([]byte, error) {
	t.Helper()
	program := filepath.Join(t.TempDir(), "derived_property")
	if runtime.GOOS == "windows" {
		program += ".exe"
	}
	build := exec.Command("go", "build", "-o", program, ".")
	build.Dir = folder
	if output, err := build.CombinedOutput(); err != nil {
		return output, err
	}
	run := exec.Command(program)
	run.Dir = dir
	return run.CombinedOutput()
}

// sections splits the output at the two headings.
func sections(text string) (configured, coded string, ok bool) {
	first := strings.Index(text, fromConfiguration)
	second := strings.Index(text, fromCode)
	if first < 0 || second < first {
		return "", "", false
	}
	return text[first:second], text[second:], true
}
