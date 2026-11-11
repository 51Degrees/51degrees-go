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

func TestGettingStarted(t *testing.T) {
	test(t, "getting_started")
}

func TestGettingStartedFreeDataFile(t *testing.T) {
	testFree(t, "getting_started")
}

func TestOfflineProcessingFreeDataFile(t *testing.T) {
	testFree(t, "offline_processing")
}

func TestReloadFromFileFreeDataFile(t *testing.T) {
	testFree(t, "reload_from_file")
}

func TestPerformanceFreeDataFile(t *testing.T) {
	testFree(t, "performance")
}

func TestUpdatePollingIntervalFreeDataFile(t *testing.T) {
	testFree(t, "update_polling_interval")
}

func TestOfflineProcessing(t *testing.T) {
	test(t, "offline_processing")
}

func TestReloadFromFile(t *testing.T) {
	test(t, "reload_from_file")
}

func TestUpdatePollingInterval(t *testing.T) {
	test(t, "update_polling_interval")
}

// The cloud example needs a Resource Key that includes IP Intelligence
// properties, so it is always built but run only when one is given.
func TestGettingStartedCloud(t *testing.T) {
	build := exec.Command("go", "build", "-o", os.DevNull, "./getting_started_cloud")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("_51DEGREES_RESOURCE_KEY") == "" {
		t.Skip("_51DEGREES_RESOURCE_KEY is not set, so the cloud example is not run")
	}
	test(t, "getting_started_cloud")
}

// The mixed examples are a separate Go module; their integration test lives
// in mixed/integration_test.go and runs via `go test` inside that module.

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
// data file, evidence, a key or an update address.
func withoutSettings(environment []string) []string {
	kept := make([]string, 0, len(environment))
	for _, variable := range environment {
		switch strings.ToUpper(strings.SplitN(variable, "=", 2)[0]) {
		case "_51DEGREES_IPI_PATH", "_51DEGREES_DD_PATH",
			"_51DEGREES_EVIDENCE_YAML", "_51DEGREES_LICENSE_KEY",
			"_51DEGREES_IPI_DATA_FILE_URL":
			continue
		}
		kept = append(kept, variable)
	}
	return kept
}
