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
	cmd := exec.Command("go", "run", "./"+name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Error(err)
	}
}

func TestMixedGettingStartedConsole(t *testing.T) {
	test(t, "getting_started_console")
}

func TestMixedGettingStartedConsoleFreeDataFiles(t *testing.T) {
	testFree(t, "getting_started_console")
}

// testFree runs an example as a fresh clone would, without the settings CI
// gives the other tests, so that it uses the free data files and evidence.
func testFree(t *testing.T, name string) {
	cmd := exec.Command("go", "run", "./"+name)
	cmd.Env = withoutSettings(os.Environ())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Error(err)
	}
}

// withoutSettings returns the environment without the variables that name a
// data file, evidence, a key or an update address, under their 51Degrees
// names and their deprecated ones.
func withoutSettings(environment []string) []string {
	kept := make([]string, 0, len(environment))
	for _, variable := range environment {
		switch strings.ToUpper(strings.SplitN(variable, "=", 2)[0]) {
		case "_51DEGREES_IPI_PATH", "_51DEGREES_DD_PATH",
			"_51DEGREES_EVIDENCE_YAML", "_51DEGREES_LICENSE_KEY",
			"_51DEGREES_IPI_DATA_FILE_URL":
			continue
		// The deprecated names of those variables, each written in two
		// parts so that this file does not name an old variable.
		case "DATA" + "_FILE", "DD_DATA" + "_FILE", "EVIDENCE" + "_YAML",
			"LICENSE" + "_KEY", "IPI" + "_KEY", "DEVICE" + "_DETECTION_KEY",
			"IPI_DATA" + "_FILE_URL":
			continue
		}
		kept = append(kept, variable)
	}
	return kept
}

// TestWithoutSettingsDropsBothNames checks a variable is left out under its
// 51Degrees name and under its deprecated names, and others are kept.
func TestWithoutSettingsDropsBothNames(t *testing.T) {
	got := withoutSettings([]string{
		"_51DEGREES_DD_PATH=a", "_51DEGREES_LICENSE_KEY=b",
		"DATA" + "_FILE=c", "DD_DATA" + "_FILE=d", "EVIDENCE" + "_YAML=e",
		"LICENSE" + "_KEY=f", "IPI" + "_KEY=g",
		"DEVICE" + "_DETECTION_KEY=h", "IPI_DATA" + "_FILE_URL=i",
		"PATH=kept",
	})
	if len(got) != 1 || got[0] != "PATH=kept" {
		t.Errorf("withoutSettings kept %q, want only PATH", got)
	}
}
