package common_go

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// pullTestPanic is what panicOnceLogger panics with.
const pullTestPanic = "pull test panic"

// panicOnceLogger records every line and panics once, on the first line that
// holds trigger, which puts a panic inside a pull.
type panicOnceLogger struct {
	mu      sync.Mutex
	lines   []string
	trigger string
	fired   bool
}

func (l *panicOnceLogger) Printf(format string, v ...interface{}) {
	line := fmt.Sprintf(format, v...)
	l.mu.Lock()
	l.lines = append(l.lines, line)
	fire := !l.fired && strings.Contains(line, l.trigger)
	l.fired = l.fired || fire
	l.mu.Unlock()
	if fire {
		panic(pullTestPanic)
	}
}

func (l *panicOnceLogger) has(part string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, line := range l.lines {
		if strings.Contains(line, part) {
			return true
		}
	}
	return false
}

// A panic in a pull is logged and the puller carries on. The pull after it
// comes after the polling interval and a random time, as after any other
// pull, so a panic in the pull on start is not followed by a second pull at
// once.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#errors
func TestScheduleFilePulling_APanicInAPullIsLoggedAndPullingCarriesOn(t *testing.T) {
	const pullEveryMs = 5
	const maxRandomizationMs = 3600000
	const pullTimeoutSeconds = 3

	var mu sync.Mutex
	var given []int
	drawRandomizationMs = func(maxMs int) int {
		mu.Lock()
		defer mu.Unlock()
		given = append(given, maxMs)
		return 0
	}
	t.Cleanup(func() { drawRandomizationMs = randomizationMs })

	dataFile := filepath.Join(t.TempDir(), "data.hash")
	if err := os.WriteFile(dataFile, []byte("current"), 0644); err != nil {
		t.Fatal(err)
	}
	server := createMockServer(http.StatusOK, nil, []byte("new"))
	defer server.Close()

	logs := &panicOnceLogger{trigger: "Checking for update from"}
	updater := NewFileUpdater(server.URL)
	updater.SetDataFile(dataFile)
	updater.SetIsFileWatcherEnabled(false)
	updater.SetUpdateOnStartEnabled(true)
	updater.SetDataFilePullEveryMs(pullEveryMs)
	updater.SetRandomization(maxRandomizationMs)
	updater.SetCustomLogger(logs)

	stopCh := make(chan *sync.WaitGroup)
	reload := make(chan struct{})
	ended := make(chan interface{}, 1)
	go func() {
		defer func() { ended <- recover() }()
		updater.ScheduleFilePulling(stopCh, reload)
	}()

	select {
	case r := <-ended:
		t.Fatalf("the puller ended at the pull on start, with %v", r)
	case <-reload:
	case <-time.After(pullTimeoutSeconds * time.Second):
		t.Fatalf("no pull within %d seconds of the panic", pullTimeoutSeconds)
	}
	stopPuller(stopCh, reload)

	if !logs.has("An unhandled error occurred while checking for automatic updates for engine 'Engine'. " +
		"Error detail: panic: " + pullTestPanic) {
		t.Error("the panic was not logged")
	}
	mu.Lock()
	got := append([]int(nil), given...)
	mu.Unlock()
	if len(got) == 0 {
		t.Fatal("no random time was drawn before the pull after the panic")
	}
	for _, maxMs := range got {
		if maxMs != maxRandomizationMs {
			t.Fatalf("the draw was given a maximum of %d, want %d",
				maxMs, maxRandomizationMs)
		}
	}
}
