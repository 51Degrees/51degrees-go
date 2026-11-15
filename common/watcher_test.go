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

package common_go

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	watcherTestPollMs    = 10
	watcherTestTimeoutMs = 2000
	// watcherChangeWaitSeconds is how long a test waits for a change to be
	// seen. The watcher saw one within 32 ms in 600 runs on Ubuntu, macOS and
	// Windows, and a loaded runner has stalled for seconds, so this is well
	// above both. A watcher that misses a change never sees it, however long
	// the test waits.
	watcherChangeWaitSeconds = 10
)

// watcherGoroutines counts the goroutines running a watcher's loop.
func watcherGoroutines() int {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	return strings.Count(string(buf[:n]), "(*Watcher).Start.func")
}

// waitUntil polls the condition until it holds or the timeout passes.
func waitUntil(condition func() bool) bool {
	deadline := time.Now().Add(watcherTestTimeoutMs * time.Millisecond)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(watcherTestPollMs * time.Millisecond)
	}
	return condition()
}

// newTestWatcher starts a watcher on a new file that checks every few
// milliseconds.
func newTestWatcher(t *testing.T) (*Watcher, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.hash")
	if err := os.WriteFile(path, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	poll := watcherTestPollMs * time.Millisecond
	w, err := newWatcher(path, poll, poll)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	return w, path
}

// A change made as soon as Start returns is seen, as the engines replace a
// data file while they run. The watcher measures a change against the file
// as it was when Start was called, so a replacement of the same size with a
// later modification time counts however soon it comes.
func TestWatcher_SeesAChangeRightAfterStart(t *testing.T) {
	// With one processor the watcher's goroutine does not run before the
	// change below is made, unless Start itself looks at the file first.
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	path := filepath.Join(t.TempDir(), "data.hash")
	if err := os.WriteFile(path, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	earlier := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, earlier, earlier); err != nil {
		t.Fatal(err)
	}
	poll := watcherTestPollMs * time.Millisecond
	w, err := newWatcher(path, poll, poll)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	if err := os.WriteFile(path, []byte("two"), 0644); err != nil {
		t.Fatal(err)
	}
	// The errors are read, as the engines read them, so that one does not
	// hold up the watcher, and each is logged.
	started := time.Now()
	timeout := time.After(watcherChangeWaitSeconds * time.Second)
	for {
		select {
		case <-w.Changed():
			t.Logf("the change was seen after %v", time.Since(started))
			return
		case err := <-w.Errors():
			t.Logf("the watcher reported %v", err)
		case <-timeout:
			t.Fatalf("a change made right after Start was not seen within %d "+
				"seconds", watcherChangeWaitSeconds)
		}
	}
}

func TestWatcher_StopEndsGoroutine(t *testing.T) {
	before := watcherGoroutines()
	w, _ := newTestWatcher(t)
	if err := w.Stop(); err != nil {
		t.Fatal(err)
	}
	if !waitUntil(func() bool { return watcherGoroutines() == before }) {
		t.Fatalf("%d watcher goroutines before Start, %d after Stop",
			before, watcherGoroutines())
	}
	select {
	case stopped := <-w.IsStopped():
		if !stopped {
			t.Error("IsStopped sent false")
		}
	default:
		t.Error("IsStopped did not report the stop")
	}
}

func TestWatcher_StopWithChangeUnread(t *testing.T) {
	w, path := newTestWatcher(t)
	time.Sleep(5 * watcherTestPollMs * time.Millisecond)
	if err := os.WriteFile(path, []byte("two!"), 0644); err != nil {
		t.Fatal(err)
	}
	// Nothing reads Changed, so the watcher is left waiting to send the
	// change when Stop is called.
	time.Sleep(20 * watcherTestPollMs * time.Millisecond)
	stopped := make(chan error, 1)
	go func() { stopped <- w.Stop() }()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(watcherTestTimeoutMs * time.Millisecond):
		t.Fatal("Stop did not return while a change was unread")
	}
}
