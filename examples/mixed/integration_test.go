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

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func test(t *testing.T, name string) {
	run(t, name, nil)
}

// run runs an example in the environment given, or in this process's when
// that is nil, prints what it printed, and returns what it printed.
func run(t *testing.T, name string, environment []string) string {
	cmd := exec.Command("go", "run", "./"+name)
	cmd.Env = environment
	output, err := cmd.CombinedOutput()
	os.Stdout.Write(output)
	if err != nil {
		t.Error(err)
	}
	return string(output)
}

func TestMixedGettingStartedConsole(t *testing.T) {
	test(t, "getting_started_console")
}

func TestMixedGettingStartedConsoleFreeDataFiles(t *testing.T) {
	testFree(t, "getting_started_console")
}

// The web example serves requests rather than ending, so its own tests run
// its handler. They build its engines from the files the settings name, so
// they are run again, as a fresh clone would run them, with no settings, and
// the engines must then be made from the free data files.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/device-detection-specification/required-examples.md?plain=1#L20-L23
func TestMixedGettingStartedWebFreeDataFiles(t *testing.T) {
	cmd := exec.Command("go", "test", "-count=1", "-v", "./getting_started_web")
	cmd.Env = withoutSettings(os.Environ())
	output, err := cmd.CombinedOutput()
	// The child's own test lines are not passed on as they are, because a
	// report that reads this test's output would take them for its own.
	if err != nil {
		t.Errorf("the web example's tests failed with no settings: %v\n%s",
			err, childLines(output))
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, " data:") {
			t.Log(strings.TrimSpace(line))
		}
	}
	for _, free := range []string{"51Degrees-LiteV4.1.hash", "51Degrees-LiteV41.ipi"} {
		if !strings.Contains(string(output), free) {
			t.Errorf("the web example's tests did not load %s", free)
		}
	}
}

// childLines gives the output of a child go test with each line marked as
// the child's.
func childLines(output []byte) string {
	lines := strings.Split(strings.TrimRight(string(output), "\n"), "\n")
	for i, line := range lines {
		lines[i] = "child: " + line
	}
	return strings.Join(lines, "\n")
}

// testFree runs an example as a fresh clone would, without the settings CI
// gives the other tests, so that it uses the free data files and evidence.
func testFree(t *testing.T, name string) {
	run(t, name, withoutSettings(os.Environ()))
}

// withoutSettings returns the environment without the variables that name a
// data file, evidence or a key.
func withoutSettings(environment []string) []string {
	kept := make([]string, 0, len(environment))
	for _, variable := range environment {
		switch strings.ToUpper(strings.SplitN(variable, "=", 2)[0]) {
		case "_51DEGREES_IPI_PATH", "_51DEGREES_DD_PATH",
			"_51DEGREES_EVIDENCE_YAML", "_51DEGREES_LICENSE_KEY":
			continue
		}
		kept = append(kept, variable)
	}
	return kept
}

// TestWithoutSettingsDropsTheNames checks each of those variables is left
// out, and others are kept.
func TestWithoutSettingsDropsTheNames(t *testing.T) {
	got := withoutSettings([]string{
		"_51DEGREES_DD_PATH=a", "_51DEGREES_LICENSE_KEY=b",
		"_51DEGREES_IPI_PATH=j", "_51DEGREES_EVIDENCE_YAML=k",
		"PATH=kept",
	})
	if len(got) != 1 || got[0] != "PATH=kept" {
		t.Errorf("withoutSettings kept %q, want only PATH", got)
	}
}
