package main

import (
	"os"
	"os/exec"
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
// properties, so it runs only when one is given.
func TestGettingStartedCloud(t *testing.T) {
	if os.Getenv("_51DEGREES_RESOURCE_KEY") == "" {
		t.Skip("_51DEGREES_RESOURCE_KEY is not set, so the cloud example is not run")
	}
	test(t, "getting_started_cloud")
}

// The mixed examples are a separate Go module; their integration test lives
// in mixed/integration_test.go and runs via `go test` inside that module.
