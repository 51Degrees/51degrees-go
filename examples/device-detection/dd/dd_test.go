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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// test runs an example, prints what it printed, checks that it ends with a
// Find out more section, and returns what it printed.
func test(t *testing.T, name string) string {
	output, err := exec.Command("go", "run", "./"+name).CombinedOutput()
	os.Stdout.Write(output)
	if err != nil {
		t.Error(err)
	}
	if err := CheckFindOutMore(string(output)); err != nil {
		t.Errorf("%s: %v", name, err)
	}
	return string(output)
}

// testEnterprise runs an example and checks that it shows the properties only
// the Enterprise data file has, with the link to ask for a copy when the data
// file in use lacks them.
func testEnterprise(t *testing.T, name string) {
	if err := CheckEnterprise(test(t, name)); err != nil {
		t.Errorf("%s: %v", name, err)
	}
}

func TestGettingStarted(t *testing.T) {
	testEnterprise(t, "getting_started")
}

func TestMatchDeviceId(t *testing.T) {
	testEnterprise(t, "match_device_id")
}

func TestMatchMetrics(t *testing.T) {
	testEnterprise(t, "match_metrics")
}

func TestOfflineProcessing(t *testing.T) {
	testEnterprise(t, "offline_processing")
}

func TestPerformance(t *testing.T) {
	testEnterprise(t, "performance")
}

func TestReloadFromFile(t *testing.T) {
	testEnterprise(t, "reload_from_file")
}

// TestStronglyTyped runs the example and checks that it reads values as their
// Go types and shows the properties only the Enterprise data file has.
func TestStronglyTyped(t *testing.T) {
	output := test(t, "strongly_typed")
	if err := CheckEnterprise(output); err != nil {
		t.Errorf("strongly_typed: %v", err)
	}
	for _, typed := range []string{"IsMobile: true (bool)", " (int)"} {
		if !strings.Contains(output, typed) {
			t.Errorf("strongly_typed does not show %q", typed)
		}
	}
}

// examples are the examples in this folder that read a data file.
var examples = []string{"getting_started", "match_device_id", "match_metrics",
	"offline_processing", "performance", "reload_from_file", "strongly_typed"}

// runWith runs an example with the environment variable name set to value,
// and returns what it printed and how it ended.
func runWith(name, variable, value string) (string, error) {
	command := exec.Command("go", "run", "./"+name)
	command.Env = append(os.Environ(), variable+"="+value)
	output, err := command.CombinedOutput()
	return string(output), err
}

// Each example reads the data file _51DEGREES_DD_PATH names, so a caller or a
// test can supply one, as the on-premise examples do. Given a file that does
// not exist, each stops rather than using the free file it would otherwise
// find.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/device-detection-specification/required-examples.md#L29-L30
func TestEachExampleReadsTheDataFileVariable(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "supplied.hash")
	for _, name := range examples {
		if output, err := runWith(name, "_51DEGREES_DD_PATH",
			missing); err == nil {
			t.Errorf("%s ran with _51DEGREES_DD_PATH naming a file that "+
				"does not exist, so it did not read the variable:\n%s",
				name, output)
		}
	}
	supplied := GetFilePathByName([]string{LiteDataFile})
	if output, err := runWith("getting_started", "_51DEGREES_DD_PATH",
		supplied); err != nil {
		t.Errorf("getting_started did not run with _51DEGREES_DD_PATH "+
			"naming the Lite file: %v\n%s", err, output)
	}
}

// The examples that read Evidence from a file read the file
// _51DEGREES_EVIDENCE_YAML names.
func TestEachExampleReadsTheEvidenceFileVariable(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "supplied.yml")
	for _, name := range []string{"offline_processing", "performance",
		"reload_from_file"} {
		if output, err := runWith(name, "_51DEGREES_EVIDENCE_YAML",
			missing); err == nil {
			t.Errorf("%s ran with _51DEGREES_EVIDENCE_YAML naming a file "+
				"that does not exist, so it did not read the variable:\n%s",
				name, output)
		}
	}
}
