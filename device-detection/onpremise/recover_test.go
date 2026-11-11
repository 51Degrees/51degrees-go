package onpremise

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	// recoverTestPanic is what the log panics with.
	recoverTestPanic = "recover test panic"
	// pullingLine starts the first line each pull logs.
	pullingLine = "Checking for update from"
	// loadedLine starts the line each load that worked logs.
	loadedLine = "data file loaded from"
)

// panicLog records every line, as reloadTestLog does, and panics once, on the
// first line holding trigger after it is armed. That puts a panic inside a
// pull or a reload without changing the engine.
type panicLog struct {
	reloadTestLog
	trigger string
	armed   atomic.Bool
	fired   atomic.Bool
}

func (l *panicLog) Printf(format string, v ...interface{}) {
	l.reloadTestLog.Printf(format, v...)
	if l.armed.Load() &&
		strings.Contains(fmt.Sprintf(format, v...), l.trigger) &&
		l.fired.CompareAndSwap(false, true) {
		panic(recoverTestPanic)
	}
}

// count returns how many lines hold part.
func (l *reloadTestLog) count(part string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, line := range l.lines {
		if strings.Contains(line, part) {
			n++
		}
	}
	return n
}

// A panic in a pull is logged, and the puller pulls again after the polling
// interval, rather than the panic ending the process.
// See https://github.com/51Degrees/specifications/blob/67ead69/pipeline-specification/features/data-updates.md#errors
func TestPanicInAPullIsLoggedAndPullingCarriesOn(t *testing.T) {
	before := engineGoroutines()
	server := newUpdateServer()
	defer server.Close()
	logs := &panicLog{trigger: pullingLine}
	e := newReloadTestEngine(t,
		WithDataFile(writeDataFile(t, readGoodFile(t))),
		WithDataUpdateUrl(server.URL),
		WithPollingInterval(1),
		WithRandomization(0),
		WithFileWatch(false),
		WithCustomLogger(logs))
	logs.armed.Store(true)

	waitFor(t, "the panic to be logged", func() bool {
		return logs.has("An unhandled error occurred while checking for automatic updates", recoverTestPanic)
	})
	pulls := logs.count(pullingLine)
	waitFor(t, "a pull after the panic", func() bool {
		return logs.count(pullingLine) > pulls
	})

	e.stop(t)
	waitFor(t, "the engine's goroutines to end", func() bool {
		return engineGoroutines() == before
	})
}

// A panic in a reload of a pulled file is logged, the engine keeps answering,
// and a later pulled file is loaded.
func TestPanicInAPulledReloadIsLoggedAndReloadingCarriesOn(t *testing.T) {
	before := engineGoroutines()
	good := readGoodFile(t)
	server := newUpdateServer()
	defer server.Close()
	logs := &panicLog{trigger: loadedLine}
	e := newReloadTestEngine(t,
		WithDataFile(writeDataFile(t, good)),
		WithDataUpdateUrl(server.URL),
		WithPollingInterval(1),
		WithRandomization(0),
		WithFileWatch(false),
		WithCustomLogger(logs))
	answer := e.isMobile(t)
	marked := markedGoodFile(t, good, e.published())
	logs.armed.Store(true)

	server.publish(good)
	waitFor(t, "the panic to be logged", func() bool {
		return logs.has("An unhandled error occurred while checking for automatic updates for engine 'DeviceDetectionHashEngine'", recoverTestPanic)
	})
	if got := e.isMobile(t); got != answer {
		t.Errorf("IsMobile is %q after the panic, was %q", got, answer)
	}
	server.publish(marked)
	waitFor(t, "the later file to be loaded", func() bool {
		return e.published().Year() == reloadTestMarkedYear
	})

	e.stop(t)
	waitFor(t, "the engine's goroutines to end", func() bool {
		return engineGoroutines() == before
	})
}

// A panic in a reload of a watched file is logged, and a later change to the
// file is loaded.
func TestPanicInAWatchedReloadIsLoggedAndWatchingCarriesOn(t *testing.T) {
	before := engineGoroutines()
	good := readGoodFile(t)
	data := writeDataFile(t, good)
	logs := &panicLog{trigger: loadedLine}
	e := newReloadTestEngine(t,
		WithDataFile(data),
		WithAutoUpdate(false),
		WithFileWatch(true),
		WithTempDataCopy(true),
		WithCustomLogger(logs))
	marked := markedGoodFile(t, good, e.published())
	logs.armed.Store(true)

	replaceFile(t, data, good)
	waitFor(t, "the panic to be logged", func() bool {
		return logs.has("An unhandled error occurred while checking for automatic updates for engine 'DeviceDetectionHashEngine'",
			recoverTestPanic)
	})
	replaceFile(t, data, marked)
	waitFor(t, "the later file to be loaded", func() bool {
		return e.published().Year() == reloadTestMarkedYear
	})

	e.stop(t)
	waitFor(t, "the engine's goroutines to end", func() bool {
		return engineGoroutines() == before
	})
}
