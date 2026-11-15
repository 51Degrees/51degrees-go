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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// panickingEngine panics in its first panics refreshes and loads every one
// after them.
type panickingEngine struct {
	mu     sync.Mutex
	tries  int
	panics int
}

func (p *panickingEngine) refresh() {
	p.mu.Lock()
	p.tries++
	tries := p.tries
	p.mu.Unlock()
	if tries <= p.panics {
		panic("the data file could not be read")
	}
}

func (p *panickingEngine) RefreshFromFile(string) error { p.refresh(); return nil }

func (p *panickingEngine) RefreshFromBytes([]byte) error { p.refresh(); return nil }

// waitFor waits until the reports hold a status of want, and fails the test
// after reportWaitSeconds.
func (r *reports) waitFor(t *testing.T, want Status) []Complete {
	t.Helper()
	deadline := time.After(reportWaitSeconds * time.Second)
	for {
		list := r.list()
		for _, c := range list {
			if c.Status == want {
				return list
			}
		}
		select {
		case <-r.done:
		case <-deadline:
			t.Fatalf("no check was reported as %q, the reports were %v", want, list)
		}
	}
}

// reportWaitSeconds is how long a test waits for a check to be reported.
const reportWaitSeconds = 10

// unhandledLine is the error table's last row, logged for a failure the table
// has no row of its own for.
const unhandledLine = `level=ERROR msg="An unhandled error occurred while ` +
	`checking for automatic updates for engine 'Eng'"`

// A failure in a check started by update polling is caught, logged and
// reported, and later checks go on, so a later good data file is applied.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/data-updates.md#errors
func TestAPanicInAPolledCheckIsReportedAndPollingGoesOn(t *testing.T) {
	srv := httptest.NewServer(&requestRecorder{status: http.StatusOK,
		body: "DATA"})
	defer srv.Close()
	svc, out := loggedService(srv.Client())
	defer svc.Close()
	got := newReports()
	if err := svc.Register(&panickingEngine{panics: 1}, "Eng", Config{
		Identifier: "a", DataUpdateURL: srv.URL, OnComplete: got.record,
		DataFilePath:            filepath.Join(t.TempDir(), "data.bin"),
		AutomaticUpdatesEnabled: true, PollingInterval: 10 * time.Millisecond,
	}); err != nil {
		t.Fatal(err)
	}

	list := got.waitFor(t, StatusSuccess)
	if list[0].Status != StatusUnknownError {
		t.Errorf("the check that panicked was reported as %q, want %q",
			list[0].Status, StatusUnknownError)
	}
	if !strings.Contains(out.String(), unhandledLine) {
		t.Errorf("the log does not hold %s:\n%s", unhandledLine, out.String())
	}
}

// A failure in a check started by the file watcher is caught, logged and
// reported, and the watcher goes on, so a later good data file is applied.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/data-updates.md#errors
func TestAPanicInAWatchedCheckIsReportedAndWatchingGoesOn(t *testing.T) {
	path := writeFile(t, "INITIAL")
	svc, out := loggedService(nil)
	defer svc.Close()
	got := newReports()
	if err := svc.Register(&panickingEngine{panics: 1}, "Eng", Config{
		Identifier: "a", DataFilePath: path, OnComplete: got.record,
		FileSystemWatcherEnabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	replace(t, path, "FIRST", time.Now().Add(time.Hour))
	got.waitFor(t, StatusUnknownError)
	replace(t, path, "SECOND", time.Now().Add(2*time.Hour))
	list := got.waitFor(t, StatusSuccess)
	if list[0].Status != StatusUnknownError {
		t.Errorf("the check that panicked was reported as %q, want %q",
			list[0].Status, StatusUnknownError)
	}
	if !strings.Contains(out.String(), unhandledLine) {
		t.Errorf("the log does not hold %s:\n%s", unhandledLine, out.String())
	}
}

// replace puts a new file with content in place of path, as an update does,
// with the modification time given.
func replace(t *testing.T, path, content string, modified time.Time) {
	t.Helper()
	next := path + ".new"
	if err := os.WriteFile(next, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(next, modified, modified); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(next, path); err != nil {
		t.Fatal(err)
	}
}
