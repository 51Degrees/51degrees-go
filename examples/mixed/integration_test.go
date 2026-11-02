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

func TestMixedGettingStartedConsole(t *testing.T) {
	test(t, "getting_started_console")
}
