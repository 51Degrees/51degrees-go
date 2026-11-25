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

package onpremise

// These tests follow what a check of a watched data file reports when the
// file is not newer than the one loaded.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/data-updates.md#newfileavailable-via-remote-or-file-watcher
// and https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/data-updates.md#reporting-each-check

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	common_go "github.com/51Degrees/51degrees-go/common/v4"
)

// How long a test waits to see a check blocked on a lock, and how often it
// looks.
const (
	waitingTimeoutSeconds   = 30
	waitingPollMilliseconds = 10
)

// updateInProgress is the status the specification names for a watched check
// that waited while another reload ran and then found nothing left to do.
const updateInProgress = common_go.UpdateStatus("AUTO_UPDATE_IN_PROGRESS")

// statuses gives the status of each report so far, in order.
func (l *reportLog) statuses() []common_go.UpdateStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	statuses := make([]common_go.UpdateStatus, 0, len(l.reports))
	for _, c := range l.reports {
		statuses = append(statuses, c.Status)
	}
	return statuses
}

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

// renameIn swaps the data file for content with the modification time given,
// with a rename, so the watcher never sees another time.
func renameIn(t *testing.T, path string, data []byte, modified time.Time) {
	t.Helper()
	next := path + ".new"
	if err := os.WriteFile(next, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(next, modified, modified); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(next, path); err != nil {
		t.Fatal(err)
	}
}

// newWatchedEngine starts an engine that watches data and reports each check
// to reports.
func newWatchedEngine(t *testing.T, data string,
	reports *reportLog) *reloadTestEngine {
	t.Helper()
	e := newReloadTestEngine(t,
		WithDataFile(data),
		WithAutoUpdate(false),
		WithFileWatch(true),
		WithTempDataCopy(true),
		WithCustomLogger(&reloadTestLog{}),
		WithUpdateCallback(reports.add))
	reports.watch(e)
	return e
}

// A watched data file that is not newer than the one the engine loaded is not
// loaded, and the check is reported as not needed.
func TestAWatchedFileThatIsNotNewerIsNotLoaded(t *testing.T) {
	good := readGoodFile(t)
	reports := &reportLog{}
	data := writeDataFile(t, good)
	e := newWatchedEngine(t, data, reports)
	loaded, err := os.Stat(data)
	if err != nil {
		t.Fatal(err)
	}
	published := e.published()

	// A file published earlier put in place changes the time the watcher
	// sees, and is not newer than the file loaded.
	renameIn(t, data, datedGoodFile(t, good, published, published.AddDate(0, 0, -1)),
		loaded.ModTime().Add(-time.Hour))
	waitFor(t, "the check to be reported", func() bool {
		return len(reports.statuses()) > 0
	})
	got := reports.statuses()
	if len(got) != 1 || got[0] != common_go.UpdateNotNeeded {
		t.Errorf("the older file was reported as %v, want [%s]", got,
			common_go.UpdateNotNeeded)
	}
	if !e.published().Equal(published) {
		t.Errorf("the engine loaded the older file, published %v, and was "+
			"published %v", e.published(), published)
	}
}

// A watched check that waited while another reload loaded the same file finds
// nothing left to do, and is reported as in progress.
func TestAWatchedCheckThatWaitedForAnotherReloadIsInProgress(t *testing.T) {
	good := readGoodFile(t)
	reports := &reportLog{}
	data := writeDataFile(t, good)
	e := newWatchedEngine(t, data, reports)
	marked := markedGoodFile(t, good, e.published())

	// Hold the reload lock so the watcher's check of the replaced file waits,
	// and load the file as another reload would.
	e.reloadMu.Lock()
	replaceFile(t, data, marked)
	waitingIn(t, "reloadOnce")
	err := e.processFileExternallyChanged()
	e.reloadMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the check to be reported", func() bool {
		return len(reports.statuses()) > 0
	})
	got := reports.statuses()
	if len(got) != 1 || got[0] != updateInProgress {
		t.Errorf("the check that waited was reported as %v, want [%s]", got,
			updateInProgress)
	}
	if year := e.published().Year(); year != reloadTestMarkedYear {
		t.Errorf("the engine answers from data published in %d, want %d",
			year, reloadTestMarkedYear)
	}
}
