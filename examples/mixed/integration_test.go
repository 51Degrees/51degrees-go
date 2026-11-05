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

func TestMixedGettingStartedConsole(t *testing.T) {
	test(t, "getting_started_console")
}
