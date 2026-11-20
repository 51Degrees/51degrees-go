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

package dataupdate

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

// How long a test waits to see a check blocked on a lock, and how often it
// looks.
const (
	waitingTimeoutSeconds   = 30
	waitingPollMilliseconds = 10
)

// waitingIn waits until a goroutine is blocked on a mutex inside the function
// named.
func waitingIn(t *testing.T, function string) {
	t.Helper()
	deadline := time.Now().Add(waitingTimeoutSeconds * time.Second)
	buf := make([]byte, 1<<20)
	for time.Now().Before(deadline) {
		stacks := string(buf[:runtime.Stack(buf, true)])
		for _, g := range strings.Split(stacks, "\n\n") {
			if strings.Contains(g, "SemacquireMutex") &&
				strings.Contains(g, function) {
				return
			}
		}
		time.Sleep(waitingPollMilliseconds * time.Millisecond)
	}
	t.Fatalf("no goroutine waited for a lock in %s", function)
}

// A watched check that waited while another check of the same file loaded it
// finds nothing left to do, and is reported as in progress, the status the
// specification names.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/data-updates.md#reporting-each-check
func TestAWatchedCheckThatWaitedForAnotherIsInProgress(t *testing.T) {
	path := writeFile(t, "INITIAL")
	eng := &fakeEngine{}
	svc := New(nil, nil)
	defer svc.Close()
	got := newReports()
	if err := svc.Register(eng, "Eng", Config{
		Identifier: "a", DataFilePath: path, OnComplete: got.record,
		FileSystemWatcherEnabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	svc.mu.Lock()
	reg := svc.registrations["a"]
	svc.mu.Unlock()

	// Hold the lock each check takes, so the watcher's check of the replaced
	// file waits, and load the file as another check would.
	reg.refreshMu.Lock()
	replace(t, path, "UPDATED", time.Now().Add(time.Hour))
	waitingIn(t, "refreshFromNewerFile")
	err := reg.loadDataFile()
	reg.refreshMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}

	got.wait(t, "the check that waited")
	want := Status("AUTO_UPDATE_IN_PROGRESS")
	list := got.list()
	if len(list) != 1 || list[0].Status != want {
		t.Errorf("the check that waited was reported as %v, want one %q",
			list, want)
	}
	if eng.lastFile() != "UPDATED" || eng.fileCount() != 1 {
		t.Errorf("the Engine loaded %q after %d refreshes, want the "+
			"replaced file loaded once", eng.lastFile(), eng.fileCount())
	}
}
