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

package ipi_onpremise

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	// failedPullIntervalSeconds is far longer than failedPullWatchSeconds, so
	// a pull after the first in that time is a retry sooner than the polling
	// interval.
	failedPullIntervalSeconds = 600
	// failedPullWatchSeconds is how long the test watches for a retry.
	failedPullWatchSeconds = 3
)

// A pull that fails is tried again after the polling interval, not a second
// later. The engine takes this from the common module it requires, so the
// test fails when that requirement names a common module without it.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#update-polling
func TestAFailedPullWaitsForThePollingInterval(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
	defer server.Close()
	logs := &reloadTestLog{}
	e := newReloadTestEngine(t,
		WithDataFile(writeDataFile(t, readGoodFile(t))),
		WithDataUpdateUrl(server.URL),
		WithPollingInterval(failedPullIntervalSeconds),
		WithRandomization(0),
		WithUpdateOnStart(true),
		WithFileWatch(false),
		WithCustomLogger(logs))

	waitFor(t, "the pull on start", func() bool {
		return logs.count(pullingLine) > 0
	})
	time.Sleep(failedPullWatchSeconds * time.Second)
	if n := logs.count(pullingLine); n != 1 {
		t.Errorf("%d pulls within %d seconds of a failed pull, want 1, as "+
			"the polling interval is %d seconds", n, failedPullWatchSeconds,
			failedPullIntervalSeconds)
	}
	e.stop(t)
}

// A reload that panics part way is not logged as loaded, so the log does not
// say the file was loaded and then that loading it failed.
func TestAReloadThatPanicsIsNotLoggedAsLoaded(t *testing.T) {
	logs := &panicLog{trigger: "failed to remove data file copy"}
	e := newReloadTestEngine(t,
		WithDataFile(writeDataFile(t, readGoodFile(t))),
		WithAutoUpdate(false),
		WithFileWatch(false),
		WithTempDataCopy(true),
		WithCustomLogger(logs))
	// The copy loaded last is swapped for a folder that is not empty, which
	// cannot be removed, so the reload logs that and the log panics.
	inUse := t.TempDir()
	if err := os.WriteFile(filepath.Join(inUse, "keep"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	e.dataFileLastUsedByManager = inUse
	loaded := logs.count(loadedLine)
	logs.armed.Store(true)

	if err := e.reload(); err == nil ||
		!strings.Contains(err.Error(), recoverTestPanic) {
		t.Fatalf("reload gave %v, want the panic", err)
	}
	if logs.count(loadedLine) != loaded {
		t.Error("a reload that panicked was logged as loaded")
	}
	e.stop(t)
}
