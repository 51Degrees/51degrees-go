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
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// recordedReports keeps each report a FileUpdater makes.
type recordedReports struct {
	mu      sync.Mutex
	reports []UpdateComplete
}

func (r *recordedReports) add(c UpdateComplete) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reports = append(r.reports, c)
}

func (r *recordedReports) all() []UpdateComplete {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]UpdateComplete{}, r.reports...)
}

// reportingUpdater is newRecordingUpdater with a callback that keeps each
// report.
func reportingUpdater(t *testing.T, url string) (*FileUpdater, *recordedLog, *recordedReports) {
	t.Helper()
	updater, log, _ := newRecordingUpdater(t, url)
	reports := &recordedReports{}
	updater.OnUpdateComplete(reports.add)
	return updater, log, reports
}

// checkOneReport fails unless the updater made exactly one report, for its
// data file, with the status given.
func checkOneReport(t *testing.T, updater *FileUpdater, reports *recordedReports, want UpdateStatus) {
	t.Helper()
	got := reports.all()
	if len(got) != 1 {
		t.Fatalf("%d reports %v, want one with %s", len(got), got, want)
	}
	if got[0].Status != want || got[0].DataFile != updater.GetDataFile() {
		t.Errorf("reported %+v, want %s for '%s'", got[0], want, updater.GetDataFile())
	}
}

// Each pull that ends without new data reports once, with the status of the
// error table row that names how it ended.
// See https://github.com/51Degrees/specifications/blob/6570727d2af6967390d3c4fbec77b9cf0540d3f3/pipeline-specification/features/data-updates.md#reporting-each-check
func TestPull_ReportsEachCheck(t *testing.T) {
	corrupt := gzipped(t, bytes.Repeat([]byte("data "), 20000))
	for i := 20; i < len(corrupt)-8; i++ {
		corrupt[i] ^= 0xff
	}
	for _, test := range []struct {
		name   string
		server func() *httptest.Server
		setUp  func(t *testing.T, updater *FileUpdater)
		want   UpdateStatus
	}{
		{name: "not modified", want: UpdateNotNeeded,
			server: func() *httptest.Server { s, _ := headerServer(http.StatusNotModified, ""); return s }},
		{name: "cannot connect", want: UpdateHTTPSError,
			server: func() *httptest.Server { s, _ := headerServer(http.StatusOK, ""); s.Close(); return s }},
		{name: "too many requests", want: UpdateTooManyRequests,
			server: func() *httptest.Server { s, _ := headerServer(http.StatusTooManyRequests, ""); return s }},
		{name: "forbidden", want: UpdateForbidden,
			server: func() *httptest.Server { s, _ := headerServer(http.StatusForbidden, ""); return s }},
		{name: "another status", want: UpdateHTTPSError,
			server: func() *httptest.Server { s, _ := headerServer(http.StatusInternalServerError, ""); return s }},
		{name: "download cut short", want: UpdateHTTPSError,
			server: func() *httptest.Server {
				return httptest.NewServer(cutShortHandler(bytes.Repeat([]byte("data "), 2000)))
			}},
		{name: "integrity check", want: UpdateMD5ValidationFailed,
			server: func() *httptest.Server {
				return createMockServer(http.StatusOK, map[string]string{"Content-MD5": "wrong"}, []byte("new"))
			}},
		{name: "corrupt compressed download", want: UpdateUnknownError,
			server: func() *httptest.Server { s, _ := headerServer(http.StatusOK, string(corrupt)); return s }},
		{name: "no data file path", want: UpdateTempPathNotSet,
			server: func() *httptest.Server { s, _ := headerServer(http.StatusOK, "new data"); return s },
			setUp:  func(_ *testing.T, updater *FileUpdater) { updater.SetDataFile("") }},
		{name: "writing the download", want: UpdateUnknownError,
			server: func() *httptest.Server { s, _ := headerServer(http.StatusOK, "new data"); return s },
			setUp: func(t *testing.T, updater *FileUpdater) {
				updater.SetDataFile(filepath.Join(t.TempDir(), "missing", "data.hash"))
			}},
		{name: "cannot replace the data file", want: UpdateCannotRename,
			server: func() *httptest.Server { s, _ := headerServer(http.StatusOK, "new data"); return s },
			setUp: func(t *testing.T, updater *FileUpdater) {
				// A directory with a file in it cannot be replaced by a rename.
				if err := os.MkdirAll(updater.GetDataFile(), 0755); err != nil {
					t.Fatal(err)
				}
				child := filepath.Join(updater.GetDataFile(), "child")
				if err := os.WriteFile(child, []byte("x"), 0644); err != nil {
					t.Fatal(err)
				}
			}},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := test.server()
			defer server.Close()
			updater, _, reports := reportingUpdater(t, server.URL)
			if test.setUp != nil {
				test.setUp(t, updater)
			}
			updater.pullRecovering(context.Background(), make(chan struct{}, 1))
			checkOneReport(t, updater, reports, test.want)
		})
	}
}

// A pull that finds new data does not report, because the check ends when the
// engine has loaded the file, and the engine reports it.
func TestPull_LeavesNewDataToTheEngineToReport(t *testing.T) {
	server, _ := headerServer(http.StatusOK, "new data")
	defer server.Close()
	updater, _, reports := reportingUpdater(t, server.URL)
	reload := make(chan struct{}, 1)
	updater.pullRecovering(context.Background(), reload)
	if len(reload) != 1 {
		t.Fatal("the new file was not announced for the engine to load")
	}
	if got := reports.all(); len(got) != 0 {
		t.Errorf("the puller reported %v for new data", got)
	}
}

// A pull that a stop cancels reports nothing, as the engine is stopping.
func TestPull_ReportsNothingWhenStopped(t *testing.T) {
	server, _ := headerServer(http.StatusNotModified, "")
	defer server.Close()
	updater, _, reports := reportingUpdater(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	updater.pullRecovering(ctx, make(chan struct{}, 1))
	if got := reports.all(); len(got) != 0 {
		t.Errorf("a cancelled pull reported %v", got)
	}
}

// A panic in a pull is the error table's last row.
func TestPull_ReportsAPanicAsAnUnknownError(t *testing.T) {
	server, _ := headerServer(http.StatusNotModified, "")
	defer server.Close()
	updater, _, reports := reportingUpdater(t, server.URL)
	updater.SetCustomLogger(&panicOnceLogger{trigger: "Checking for update from"})
	updater.pullRecovering(context.Background(), make(chan struct{}, 1))
	checkOneReport(t, updater, reports, UpdateUnknownError)
}

// A callback that panics during update polling is logged with the error
// table's last row, the callbacks after it are not called, and the next pull
// comes after the polling interval.
func TestPull_LogsACallbackThatPanics(t *testing.T) {
	server, _ := headerServer(http.StatusNotModified, "")
	defer server.Close()
	updater, log, reports := reportingUpdater(t, server.URL)
	later := &recordedReports{}
	updater.OnUpdateComplete(func(UpdateComplete) { panic("callback test panic") })
	updater.OnUpdateComplete(later.add)
	next := updater.pullRecovering(context.Background(), make(chan struct{}, 1))
	if next != updater.GetDataFilePullEveryMs() {
		t.Errorf("next pull in %d ms, want the polling interval of %d ms", next, updater.GetDataFilePullEveryMs())
	}
	checkOneReport(t, updater, reports, UpdateNotNeeded)
	if got := later.all(); len(got) != 0 {
		t.Errorf("a callback after the one that panicked heard %v", got)
	}
	want := "ERROR An unhandled error occurred while checking for automatic updates for engine " +
		"'TestEngine'. Error detail: panic in an update callback: callback test panic"
	if !log.has(want) {
		t.Errorf("missing log entry %q in:\n%s", want, log)
	}
}

// The update on start-up reports as a pull does, and passes a callback's
// panic to its caller.
func TestPullOnStart_Reports(t *testing.T) {
	t.Run("not modified", func(t *testing.T) {
		server, _ := headerServer(http.StatusNotModified, "")
		defer server.Close()
		updater, _, reports := reportingUpdater(t, server.URL)
		if _, err := updater.PullOnStart(); err != nil {
			t.Fatal(err)
		}
		checkOneReport(t, updater, reports, UpdateNotNeeded)
	})
	t.Run("failed with a local data file", func(t *testing.T) {
		server, _ := headerServer(http.StatusInternalServerError, "")
		defer server.Close()
		updater, _, reports := reportingUpdater(t, server.URL)
		if err := os.WriteFile(updater.GetDataFile(), []byte("current"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := updater.PullOnStart(); err != nil {
			t.Fatal(err)
		}
		checkOneReport(t, updater, reports, UpdateHTTPSError)
	})
	t.Run("callback panics", func(t *testing.T) {
		server, _ := headerServer(http.StatusNotModified, "")
		defer server.Close()
		updater, _, _ := reportingUpdater(t, server.URL)
		updater.OnUpdateComplete(func(UpdateComplete) { panic("callback test panic") })
		_, err := updater.PullOnStart()
		var p *UpdateCallbackPanic
		if !errors.As(err, &p) {
			t.Errorf("PullOnStart gave %v, want the callback's panic", err)
		}
	})
}

// ReportUpdateComplete calls each callback in turn and stops at one that
// panics, returning its panic.
func TestReportUpdateComplete_StopsAtACallbackThatPanics(t *testing.T) {
	updater := NewFileUpdater("http://localhost")
	updater.SetDataFile("data.hash")
	first, last := &recordedReports{}, &recordedReports{}
	updater.OnUpdateComplete(first.add)
	updater.OnUpdateComplete(func(UpdateComplete) { panic("callback test panic") })
	updater.OnUpdateComplete(last.add)
	err := updater.ReportUpdateComplete(UpdateSuccess)
	var p *UpdateCallbackPanic
	if !errors.As(err, &p) || p.Value != "callback test panic" {
		t.Errorf("ReportUpdateComplete gave %v, want the callback's panic", err)
	}
	if got := first.all(); len(got) != 1 || got[0] != (UpdateComplete{DataFile: "data.hash", Status: UpdateSuccess}) {
		t.Errorf("the first callback heard %v", got)
	}
	if got := last.all(); len(got) != 0 {
		t.Errorf("the callback after the panic heard %v", got)
	}
}
