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
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	dd_example "github.com/51Degrees/51degrees-go/examples/device-detection/dd"
)

// run runs an example in the environment given, or in this process's when
// that is nil, prints what it printed and returns it.
func run(t *testing.T, name string, environment []string) string {
	cmd := exec.Command("go", "run", "./"+name)
	cmd.Env = environment
	output, err := cmd.CombinedOutput()
	os.Stdout.Write(output)
	if err != nil {
		t.Error(err)
	}
	if err := dd_example.CheckFindOutMore(string(output)); err != nil {
		t.Errorf("%s: %v", name, err)
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
	testFree(t, "match_device_id")
}

func TestMatchMetrics(t *testing.T) {
	testEnterprise(t, "match_metrics")
}

func TestMatchMetricsFreeDataFile(t *testing.T) {
	testFree(t, "match_metrics")
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
	testFree(t, "offline_processing")
}

func TestPerformance(t *testing.T) {
	testEnterprise(t, "performance")
}

func TestPerformanceFreeDataFile(t *testing.T) {
	testFree(t, "performance")
}

func TestReloadFromFile(t *testing.T) {
	testEnterprise(t, "reload_from_file")
}

func TestReloadFromFileFreeDataFile(t *testing.T) {
	testFree(t, "reload_from_file")
}

// The update example renames each newer data file it downloads over the data
// file it is given. The other examples, tested at the same time, read the
// file the settings name as the free data file, so the example must leave
// that file as it was. It downloads only with a licence key.
func TestUpdatePollingInterval(t *testing.T) {
	environment, err := withOwnDataFile(os.Environ(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	shared := os.Getenv("_51DEGREES_DD_PATH")
	if shared == "" || os.Getenv("_51DEGREES_LICENSE_KEY") == "" {
		run(t, "update_polling_interval", environment)
		return
	}
	before := fileSum(t, shared)
	run(t, "update_polling_interval", environment)
	if fileSum(t, shared) != before {
		size := int64(-1)
		if info, err := os.Stat(shared); err == nil {
			size = info.Size()
		}
		t.Errorf("the update example replaced %s, which the other examples "+
			"read as the free data file, with a file of %d bytes", shared,
			size)
	}
}

// withOwnDataFile returns environment with _51DEGREES_DD_PATH naming a copy,
// in dir, of the data file the environment names under that name or its
// deprecated one, or of the free data file the examples find when it names
// none, and without the deprecated name. It returns environment unchanged
// when it names no data file and the free one is not found. The update
// example renames each newer data file it downloads over the one it is
// given, so with a copy it changes no file that another test reads.
func withOwnDataFile(environment []string, dir string) ([]string, error) {
	deprecated := "DATA" + "_FILE"
	var named, old string
	kept := make([]string, 0, len(environment)+1)
	for _, variable := range environment {
		name, value, _ := strings.Cut(variable, "=")
		switch strings.ToUpper(name) {
		case "_51DEGREES_DD_PATH":
			named = value
			continue
		case deprecated:
			old = value
			continue
		}
		kept = append(kept, variable)
	}
	source := named
	if source == "" {
		source = old
	}
	if source == "" {
		// A free file that is neither found nor downloaded is left for the
		// example to report, as it does when run on its own.
		source = dd_example.GetDataFile(dd_example.LiteDataFile)
		if _, err := os.Stat(source); err != nil {
			return environment, nil
		}
	}
	copied := filepath.Join(dir, filepath.Base(source))
	if err := copyFile(source, copied); err != nil {
		return nil, fmt.Errorf("the update example needs its own copy of "+
			"the data file %s: %w", source, err)
	}
	return append(kept, "_51DEGREES_DD_PATH="+copied), nil
}

// copyFile copies the file at from to a new file at to.
func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// fileSum gives the SHA-256 of the file at path, failing the test when it
// cannot be read.
func fileSum(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(content)
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
		"_51DEGREES_IPI_PATH=j", "_51DEGREES_EVIDENCE_YAML=k",
		"_51DEGREES_IPI_DATA_FILE_URL=l",
		"DATA" + "_FILE=c", "DD_DATA" + "_FILE=d", "EVIDENCE" + "_YAML=e",
		"LICENSE" + "_KEY=f", "IPI" + "_KEY=g",
		"DEVICE" + "_DETECTION_KEY=h", "IPI_DATA" + "_FILE_URL=i",
		"PATH=kept",
	})
	if len(got) != 1 || got[0] != "PATH=kept" {
		t.Errorf("withoutSettings kept %q, want only PATH", got)
	}
}
