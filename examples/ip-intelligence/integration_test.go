package main

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

func test(t *testing.T, name string) {
	if runtime.GOOS == "windows" {
		// TODO: Windows CI runner can't handle the InMemory profile, and LowMemory is
		// unstable in CI on any OS, so the integration tests are disabled on Windows.
		t.Skip(name + " currently fails on Windows, skipping")
	}
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

func TestOfflineProcessing(t *testing.T) {
	test(t, "offline_processing")
}

func TestReloadFromFile(t *testing.T) {
	test(t, "reload_from_file")
}

func TestUpdatePollingInterval(t *testing.T) {
	test(t, "update_polling_interval")
}

// The mixed examples are a separate Go module; their integration test lives
// in mixed/integration_test.go and runs via `go test` inside that module.
