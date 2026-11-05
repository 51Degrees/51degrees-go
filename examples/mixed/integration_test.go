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
// data file, evidence, a key or an update address.
func withoutSettings(environment []string) []string {
	kept := make([]string, 0, len(environment))
	for _, variable := range environment {
		switch strings.ToUpper(strings.SplitN(variable, "=", 2)[0]) {
		case "DATA_FILE", "DD_DATA_FILE", "EVIDENCE_YAML", "LICENSE_KEY",
			"DEVICE_DETECTION_KEY", "IPI_KEY", "IPI_DATA_FILE_URL":
			continue
		}
		kept = append(kept, variable)
	}
	return kept
}
