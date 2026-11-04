package ipi_onpremise

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
)

const (
	leakCheckCollections    = 3
	leakCheckPollMs         = 50
	leakCheckTimeoutSeconds = 10
)

// TestMain fails the run when a test leaves native resources behind. A running
// engine keeps its data set in native memory until the process ends, so every
// engine a test creates must be stopped. Collecting garbage first makes the
// finalizer of any results, evidence or manager a test did not free panic,
// which also fails the run.
func TestMain(m *testing.M) {
	code := m.Run()

	for i := 0; i < leakCheckCollections; i++ {
		runtime.GC()
		time.Sleep(leakCheckPollMs * time.Millisecond)
	}

	deadline := time.Now().Add(leakCheckTimeoutSeconds * time.Second)
	running := engineGoroutines()
	for running > 0 && time.Now().Before(deadline) {
		time.Sleep(leakCheckPollMs * time.Millisecond)
		running = engineGoroutines()
	}
	if running > 0 {
		fmt.Fprintf(os.Stderr,
			"%d engine goroutines are still running, so a test did not "+
				"stop an engine it created.\n", running)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}
