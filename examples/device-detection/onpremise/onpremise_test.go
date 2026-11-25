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

package onpremise

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	dd_example "github.com/51Degrees/51degrees-go/examples/device-detection/dd"
)

// run runs an example with the arguments given, in the environment given,
// or in this process's when that is nil, prints what it printed and returns
// it.
func run(t *testing.T, name string, environment []string,
	arguments ...string) string {
	cmd := exec.Command("go", append([]string{"run", "./" + name},
		arguments...)...)
	cmd.Env = environment
	output, err := cmd.CombinedOutput()
	os.Stdout.Write(output)
	if err != nil {
		t.Error(err)
	}
	return string(output)
}

func test(t *testing.T, name string) {
	run(t, name, nil)
}

// testEnterprise runs an example and checks that it shows the properties only
// the Enterprise data file has, with the link to ask for a copy when the data
// file in use lacks them.
func testEnterprise(t *testing.T, name string) {
	checkEnterprise(t, name, run(t, name, nil))
}

// testFree runs an example as a fresh clone would, without the settings CI
// gives the other tests, so that it uses the free data files and evidence,
// and checks it shows the properties only the Enterprise data file has.
func testFree(t *testing.T, name string) {
	checkEnterprise(t, name, run(t, name, withoutSettings(os.Environ())))
}

func checkEnterprise(t *testing.T, name, output string) {
	if err := dd_example.CheckEnterprise(output); err != nil {
		t.Errorf("%s: %v", name, err)
	}
}

func TestGettingStarted(t *testing.T) {
	testEnterprise(t, "getting_started")
}

func TestGettingStartedFreeDataFile(t *testing.T) {
	testFree(t, "getting_started")
}

func TestUpdatePollingIntervalFreeDataFile(t *testing.T) {
	testFree(t, "update_polling_interval")
}

func TestMatchDeviceId(t *testing.T) {
	testEnterprise(t, "match_device_id")
}

func TestMatchDeviceIdFreeDataFile(t *testing.T) {
	testFreeEndsWithFindOutMore(t, "match_device_id")
}

func TestMatchMetrics(t *testing.T) {
	testEnterprise(t, "match_metrics")
}

func TestMatchMetricsFreeDataFile(t *testing.T) {
	testFreeEndsWithFindOutMore(t, "match_metrics")
}

func TestMetadata(t *testing.T) {
	testEnterprise(t, "metadata")
}

// The Metadata example runs on the free data file and lists the Evidence
// keys, each component with its properties and their values, and the values
// of the profiles an iPhone is matched to, then shows what only the
// Enterprise data file has.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/device-detection-specification/required-examples.md?plain=1#L288-L292
func TestMetadataFreeDataFile(t *testing.T) {
	output := run(t, "metadata", withoutSettings(os.Environ()))
	checkEnterprise(t, "metadata", output)
	checkFindOutMore(t, "metadata", output)
	for _, want := range []string{
		"\theader.",
		"Component HardwarePlatform",
		"\t\tValues: ",
		"is matched to the profiles",
		"\t\tIsMobile True",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("metadata prints no %q", strings.TrimSpace(want))
		}
	}
}

func TestOfflineProcessing(t *testing.T) {
	testEnterprise(t, "offline_processing")
}

func TestOfflineProcessingFreeDataFile(t *testing.T) {
	testFreeEndsWithFindOutMore(t, "offline_processing")
}

func TestPerformance(t *testing.T) {
	testEnterprise(t, "performance")
}

func TestPerformanceFreeDataFile(t *testing.T) {
	testFreeEndsWithFindOutMore(t, "performance")
}

func TestReloadFromFile(t *testing.T) {
	testEnterprise(t, "reload_from_file")
}

func TestReloadFromFileFreeDataFile(t *testing.T) {
	testFreeOnOwnCopy(t, "reload_from_file")
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
