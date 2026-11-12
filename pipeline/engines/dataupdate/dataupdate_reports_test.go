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
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// reports collects what the service tells the callbacks a check ended with.
type reports struct {
	mu   sync.Mutex
	all  []Complete
	done chan struct{}
}

func newReports() *reports { return &reports{done: make(chan struct{}, 16)} }

func (r *reports) record(c Complete) {
	r.mu.Lock()
	r.all = append(r.all, c)
	r.mu.Unlock()
	select {
	case r.done <- struct{}{}:
	default:
	}
}

func (r *reports) list() []Complete {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Complete(nil), r.all...)
}

// countingEngine fails the first failures refreshes from a file and counts
// every try.
type countingEngine struct {
	mu       sync.Mutex
	tries    int
	failures int
}

func (c *countingEngine) RefreshFromFile(string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tries++
	if c.tries <= c.failures {
		return errors.New("the data file is held by another process")
	}
	return nil
}

func (c *countingEngine) RefreshFromBytes([]byte) error { return nil }

func (c *countingEngine) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tries
}

// writeFile writes content to a new data file and returns its path.
func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.bin")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The automatic update master switch governs the file system watcher as well
// as polling, so with automatic updates off no watcher is created and a
// replaced file is not loaded.
// See https://github.com/51Degrees/specifications/blob/a0a7008/pipeline-specification/features/data-updates.md#automatic-update-master-switch
func TestTheWatcherRunsOnlyWithAutomaticUpdatesOn(t *testing.T) {
	for _, test := range []struct {
		name      string
		automatic bool
		wantWatch bool
	}{
		{name: "automatic updates on", automatic: true, wantWatch: true},
		{name: "automatic updates off", automatic: false, wantWatch: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writeFile(t, "INITIAL")
			eng := &fakeEngine{}
			svc, out := loggedService(nil)
			defer svc.Close()
			if err := svc.Register(eng, "Eng", Config{
				Identifier: "a", DataFilePath: path,
				AutomaticUpdatesEnabled:  test.automatic,
				FileSystemWatcherEnabled: true,
			}); err != nil {
				t.Fatal(err)
			}
			created := strings.Contains(out.String(), "Creating file system watcher")
			if created != test.wantWatch {
				t.Errorf("a file system watcher was created = %v, want %v", created, test.wantWatch)
			}

			next := path + ".new"
			if err := os.WriteFile(next, []byte("REPLACED"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(next, path); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(3 * time.Second)
			for eng.lastFile() != "REPLACED" && time.Now().Before(deadline) {
				time.Sleep(20 * time.Millisecond)
			}
			if loaded := eng.lastFile() == "REPLACED"; loaded != test.wantWatch {
				t.Errorf("the replaced file was loaded = %v, want %v", loaded, test.wantWatch)
			}
		})
	}
}

// Each check that ends is reported to the callbacks with one status, and the
// status names are those of the .NET data update service.
// See https://github.com/51Degrees/specifications/blob/a0a7008/pipeline-specification/features/data-updates.md#reporting-each-check
func TestEachCheckIsReportedWithItsStatus(t *testing.T) {
	for _, test := range []struct {
		name   string
		server *requestRecorder
		engine Refreshable
		want   Status
	}{
		{name: "new data", server: &requestRecorder{status: http.StatusOK, body: "DATA"},
			want: StatusSuccess},
		{name: "no newer data", server: &requestRecorder{status: http.StatusNotModified},
			want: StatusNotNeeded},
		{name: "too many requests", server: &requestRecorder{status: http.StatusTooManyRequests},
			want: StatusTooManyRequests},
		{name: "forbidden", server: &requestRecorder{status: http.StatusForbidden},
			want: StatusForbidden},
		{name: "another status", server: &requestRecorder{status: http.StatusInternalServerError},
			want: StatusHTTPSError},
		{name: "integrity check", server: &requestRecorder{status: http.StatusOK, body: "DATA",
			header: http.Header{"Content-Md5": {"wrong"}}},
			want: StatusMD5ValidationFailed},
		{name: "the engine will not load the data",
			server: &requestRecorder{status: http.StatusOK, body: "DATA"},
			engine: &failingEngine{}, want: StatusRefreshFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			srv := httptest.NewServer(test.server)
			defer srv.Close()
			var engine Refreshable = &fakeEngine{}
			if test.engine != nil {
				engine = test.engine
			}
			svc := New(nil, srv.Client())
			defer svc.Close()
			if err := svc.Register(engine, "Eng", Config{Identifier: "a", DataUpdateURL: srv.URL,
				DataFilePath: filepath.Join(t.TempDir(), "data.bin")}); err != nil {
				t.Fatal(err)
			}
			got := newReports()
			svc.OnUpdate("a", got.record)

			_ = svc.CheckForUpdate("a")

			list := got.list()
			if len(list) != 1 {
				t.Fatalf("the check was reported %d times, want once: %v", len(list), list)
			}
			if list[0].Status != test.want {
				t.Errorf("the check was reported as %q, want %q", list[0].Status, test.want)
			}
			if list[0].Identifier != "a" {
				t.Errorf("the report names %q, want the identifier of the data file", list[0].Identifier)
			}
		})
	}
}

// Update polling reports each check it makes, so a failure reaches the
// callbacks as well as the log.
func TestPollingReportsItsFailures(t *testing.T) {
	srv := httptest.NewServer(&requestRecorder{status: http.StatusForbidden})
	defer srv.Close()
	svc, _ := loggedService(srv.Client())
	defer svc.Close()
	if err := svc.Register(&fakeEngine{}, "Eng", Config{
		Identifier: "a", DataUpdateURL: srv.URL,
		DataFilePath:            filepath.Join(t.TempDir(), "data.bin"),
		AutomaticUpdatesEnabled: true, PollingInterval: 10 * time.Millisecond,
	}); err != nil {
		t.Fatal(err)
	}
	got := newReports()
	svc.OnUpdate("a", got.record)

	select {
	case <-got.done:
	case <-time.After(5 * time.Second):
		t.Fatal("update polling reported no check")
	}
	if list := got.list(); list[0].Status != StatusForbidden {
		t.Errorf("the check was reported as %q, want %q", list[0].Status, StatusForbidden)
	}
}

// The check made on start-up is reported like any other, to the callback the
// configuration carries, which is registered before the check runs.
// See https://github.com/51Degrees/specifications/blob/a0a7008/pipeline-specification/features/data-updates.md#reporting-each-check
func TestTheCheckOnStartUpIsReported(t *testing.T) {
	srv := httptest.NewServer(&requestRecorder{status: http.StatusOK, body: "DATA"})
	defer srv.Close()
	svc := New(nil, srv.Client())
	defer svc.Close()
	got := newReports()
	if err := svc.Register(&fakeEngine{}, "Eng", Config{
		Identifier: "a", DataFilePath: filepath.Join(t.TempDir(), "data.bin"),
		DataUpdateURL: srv.URL, UpdateOnStartup: true, OnUpdate: got.record,
	}); err != nil {
		t.Fatal(err)
	}
	list := got.list()
	if len(list) != 1 || list[0].Status != StatusSuccess {
		t.Errorf("the check on start-up was reported as %v, want one report of %q", list, StatusSuccess)
	}
}

// An update from memory gives its result to its caller alone, so it reports
// nothing to the callbacks.
// See https://github.com/51Degrees/specifications/blob/a0a7008/pipeline-specification/features/data-updates.md#reporting-each-check
func TestAnUpdateFromMemoryReportsOnlyToItsCaller(t *testing.T) {
	eng := &fakeEngine{}
	svc := New(nil, nil)
	defer svc.Close()
	if err := svc.Register(eng, "Eng", Config{Identifier: "a"}); err != nil {
		t.Fatal(err)
	}
	got := newReports()
	svc.OnUpdate("a", got.record)

	if err := svc.UpdateFromMemory("a", []byte("NEW DATA")); err != nil {
		t.Fatalf("UpdateFromMemory gave %v, want no error", err)
	}
	if eng.bytesCount() != 1 {
		t.Errorf("the Engine was refreshed %d times, want once", eng.bytesCount())
	}
	if list := got.list(); len(list) != 0 {
		t.Errorf("an update from memory reported %v, want nothing", list)
	}
}

// A refresh from the data file is tried ten times, as .NET and Java do,
// because the file can be held by another process for a moment. The message
// of a refresh that never worked names the tries made.
// See https://github.com/51Degrees/specifications/blob/a0a7008/pipeline-specification/features/data-updates.md#errors
func TestARefreshFromTheDataFileIsTriedTenTimes(t *testing.T) {
	t.Run("a refresh that works on the last try", func(t *testing.T) {
		eng := &countingEngine{failures: refreshTries - 1}
		svc := New(nil, nil)
		defer svc.Close()
		if err := svc.Register(eng, "Eng", Config{
			Identifier: "a", DataFilePath: writeFile(t, "DATA")}); err != nil {
			t.Fatal(err)
		}
		if err := svc.CheckForUpdate("a"); err != nil {
			t.Errorf("CheckForUpdate gave %v, want no error after a refresh that worked", err)
		}
		if got := eng.count(); got != refreshTries {
			t.Errorf("the refresh was tried %d times, want %d", got, refreshTries)
		}
	})
	t.Run("a refresh that never works", func(t *testing.T) {
		eng := &countingEngine{failures: refreshTries + 1}
		svc := New(nil, nil)
		defer svc.Close()
		if err := svc.Register(eng, "Eng", Config{
			Identifier: "a", DataFilePath: writeFile(t, "DATA")}); err != nil {
			t.Fatal(err)
		}
		err := svc.CheckForUpdate("a")
		want := "An error occurred when applying a data update to engine 'Eng' after 10 tries."
		if err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("CheckForUpdate gave %v, want an error starting %q", err, want)
		}
		if got := eng.count(); got != refreshTries {
			t.Errorf("the refresh was tried %d times, want %d", got, refreshTries)
		}
	})
}

// The settings a data source starts with are those of the Defaults table,
// which the file updater the native Engines use starts with too.
// See https://github.com/51Degrees/specifications/blob/a0a7008/pipeline-specification/features/data-updates.md#defaults
func TestDefaultConfigHoldsTheDefaultsOfTheSpecification(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.AutomaticUpdatesEnabled {
		t.Error("automatic updates are off by default, want on")
	}
	if !cfg.FileSystemWatcherEnabled {
		t.Error("the file system watcher is off by default, want on")
	}
	if cfg.UpdateOnStartup {
		t.Error("the update on start-up is on by default, want off")
	}
	if !cfg.UseIfModifiedSince {
		t.Error("If-Modified-Since is not sent by default, want it sent")
	}
	if cfg.PollingInterval != 30*time.Minute {
		t.Errorf("the polling interval is %v by default, want 30 minutes", cfg.PollingInterval)
	}
	if cfg.MaxRandomization != 10*time.Minute {
		t.Errorf("the randomization is %v by default, want up to 10 minutes", cfg.MaxRandomization)
	}
}
