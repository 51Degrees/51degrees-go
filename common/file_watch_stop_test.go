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
	"sync"
	"testing"
	"time"
)

// A FileWatcher watching a file sets the callback, and once stopped its
// watcher's goroutine ends, so no watcher keeps running through later tests.
func TestFileWatcher_StopEndsTheWatcher(t *testing.T) {
	before := watcherGoroutines()
	logger := &LogWrapper{}
	stopCh := make(chan *sync.WaitGroup, 1)

	path := filepath.Join(t.TempDir(), "data.hash")
	if err := os.WriteFile(path, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	poll := watcherTestPollMs * time.Millisecond
	watcher, err := newWatcher(path, poll, poll)
	if err != nil {
		t.Fatal(err)
	}
	fw := &FileWatcher{
		watcher:  watcher,
		logger:   logger,
		stopCh:   stopCh,
		callback: func() {},
	}

	callbackCalled := false
	if err := fw.watch(func() { callbackCalled = true }); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	fw.callback()
	if !callbackCalled {
		t.Error("Expected callback to be set correctly")
	}

	if err := fw.stop(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-watcher.IsStopped():
	case <-time.After(watcherTestTimeoutMs * time.Millisecond):
		t.Fatal("the watcher did not stop")
	}

	// A watcher left running here keeps running through every later test.
	if !waitUntil(func() bool { return watcherGoroutines() == before }) {
		t.Errorf("%d watcher goroutines before the test, %d after it",
			before, watcherGoroutines())
	}
}
