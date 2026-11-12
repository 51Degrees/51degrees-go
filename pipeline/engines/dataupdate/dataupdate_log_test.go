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
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// requestRecorder answers every request with the status and records the
// If-Modified-Since header of each.
type requestRecorder struct {
	mu     sync.Mutex
	since  []string
	status int
	header http.Header
	body   string
}

func (r *requestRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	r.since = append(r.since, req.Header.Get("If-Modified-Since"))
	r.mu.Unlock()
	for k, v := range r.header {
		w.Header()[k] = v
	}
	w.WriteHeader(r.status)
	w.Write([]byte(r.body))
}

func (r *requestRecorder) requests() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.since...)
}

// syncBuffer is a log destination that several goroutines may write.
type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func loggedService(client *http.Client) (*Service, *syncBuffer) {
	out := &syncBuffer{}
	return New(slog.New(slog.NewTextHandler(out, nil)), client), out
}

// line is how the text handler writes a message at a level.
func line(level, msg string) string { return "level=" + level + ` msg="` + msg + `"` }

// The Logging section of the specification gives the messages of a check and
// the levels of the errors.
func TestLogsTheMessagesOfTheSpecification(t *testing.T) {
	t.Run("a download", func(t *testing.T) {
		srv := httptest.NewServer(&requestRecorder{status: http.StatusOK, body: "DATA"})
		defer srv.Close()
		svc, out := loggedService(srv.Client())
		defer svc.Close()
		path := filepath.Join(t.TempDir(), "data.bin")
		if err := svc.Register(&fakeEngine{}, "Eng", Config{
			Identifier: "a", DataFilePath: path, DataUpdateURL: srv.URL,
			UpdateOnStartup: true, AutomaticUpdatesEnabled: true, FileSystemWatcherEnabled: true,
		}); err != nil {
			t.Fatal(err)
		}
		got := out.String()
		for _, want := range []string{
			line("INFO", "Updating on start-up"),
			line("INFO", "Checking for update"),
			line("INFO", "Checking for update from '"+srv.URL+"' for Engine 'Eng'"),
			line("INFO", "Downloaded new data from '"+srv.URL+"' for Engine 'Eng'"),
			line("INFO", "Attempting to refresh Engine 'Eng' with new data"),
			line("INFO", "Creating file system watcher"),
		} {
			if !strings.Contains(got, want) {
				t.Errorf("missing %s in:\n%s", want, got)
			}
		}
	})
	t.Run("no newer data", func(t *testing.T) {
		srv := httptest.NewServer(&requestRecorder{status: http.StatusNotModified})
		defer srv.Close()
		svc, out := loggedService(srv.Client())
		defer svc.Close()
		if err := svc.Register(&fakeEngine{}, "Eng", Config{Identifier: "a", DataUpdateURL: srv.URL}); err != nil {
			t.Fatal(err)
		}
		if err := svc.CheckForUpdate("a"); err != nil {
			t.Fatal(err)
		}
		want := "level=INFO msg=\"No data newer than the current data found at '" + srv.URL + "' for Engine 'Eng'\""
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	})
}

// A failed check is given the error table's message, which follows .NET. A
// check the caller starts passes the error to the caller and logs nothing,
// and a check the poller starts logs it at Error as "Error during check for
// data update" with the error attached.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#errors
func TestFailedChecksAreReportedAsTheErrorTableGivesThem(t *testing.T) {
	refused := httptest.NewServer(http.NotFoundHandler())
	refusedURL := refused.URL
	refused.Close()
	for _, test := range []struct {
		name   string
		url    string
		server *requestRecorder
		want   string
	}{
		{name: "cannot connect", url: refusedURL,
			want: "Error accessing data update service at '%s' for engine 'Eng'. Error detail: "},
		{name: "too many requests", server: &requestRecorder{status: http.StatusTooManyRequests},
			want: "Too many requests to '%s' for engine 'Eng'"},
		{name: "forbidden", server: &requestRecorder{status: http.StatusForbidden},
			want: "Access denied to data update service at '%s' for engine 'Eng'"},
		{name: "another status", server: &requestRecorder{status: http.StatusInternalServerError},
			want: "HTTP status code '500' from data update service at '%s' for engine 'Eng'"},
		{name: "integrity check", server: &requestRecorder{status: http.StatusOK, body: "DATA",
			header: http.Header{"Content-Md5": {"wrong"}}},
			want: "Integrity check failed. MD5 hash in HTTP response 'wrong' for 'Eng'data update does " +
				"not match calculated hash for the downloaded file '"},
	} {
		t.Run(test.name, func(t *testing.T) {
			url := test.url
			if test.server != nil {
				srv := httptest.NewServer(test.server)
				defer srv.Close()
				url = srv.URL
			}
			want := test.want
			if strings.Contains(want, "%s") {
				want = fmt.Sprintf(want, url)
			}

			svc, out := loggedService(nil)
			defer svc.Close()
			if err := svc.Register(&fakeEngine{}, "Eng", Config{Identifier: "a", DataUpdateURL: url,
				DataFilePath: filepath.Join(t.TempDir(), "data.bin")}); err != nil {
				t.Fatal(err)
			}
			err := svc.CheckForUpdate("a")
			if err == nil || !strings.HasPrefix(err.Error(), want) {
				t.Errorf("CheckForUpdate gave %v, want an error starting %q", err, want)
			}
			if got := out.String(); strings.Contains(got, "level=ERROR") ||
				strings.Contains(got, "level=WARN") {
				t.Errorf("a check the caller started logged its error:\n%s", got)
			}

			polled, pollOut := loggedService(nil)
			defer polled.Close()
			if err := polled.Register(&fakeEngine{}, "Eng", Config{
				Identifier: "a", DataUpdateURL: url, AutomaticUpdatesEnabled: true,
				PollingInterval: 10 * time.Millisecond,
				DataFilePath:    filepath.Join(t.TempDir(), "data.bin"),
			}); err != nil {
				t.Fatal(err)
			}
			logged := `level=ERROR msg="Error during check for data update" error="` + want
			deadline := time.Now().Add(5 * time.Second)
			for !strings.Contains(pollOut.String(), logged) && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			if !strings.Contains(pollOut.String(), logged) {
				t.Errorf("missing %s in:\n%s", logged, pollOut)
			}
		})
	}
	t.Run("no MD5 hash", func(t *testing.T) {
		srv := httptest.NewServer(&requestRecorder{status: http.StatusOK, body: "DATA"})
		defer srv.Close()
		svc, out := loggedService(srv.Client())
		defer svc.Close()
		if err := svc.Register(&fakeEngine{}, "Eng", Config{Identifier: "a", DataUpdateURL: srv.URL}); err != nil {
			t.Fatal(err)
		}
		_ = svc.CheckForUpdate("a")
		want := line("WARN", "No MD5 hash included in data update response for '"+srv.URL+
			"'. Unable to verify data integrity")
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	})
	t.Run("applying an update from memory", func(t *testing.T) {
		svc, out := loggedService(nil)
		defer svc.Close()
		if err := svc.Register(&failingEngine{}, "Eng", Config{Identifier: "a"}); err != nil {
			t.Fatal(err)
		}
		err := svc.UpdateFromMemory("a", []byte("x"))
		want := "An error occurred when applying a data update to engine 'Eng'. Error detail: boom"
		if err == nil || err.Error() != want {
			t.Errorf("UpdateFromMemory gave %v, want %q", err, want)
		}
		if strings.Contains(out.String(), "level=ERROR") {
			t.Errorf("an update from memory logged its error:\n%s", out)
		}
	})
}

// A callback that panics stops the callbacks after it, as the Errors section
// says. A check the caller started passes the panic to the caller, and update
// polling logs it with the error table's last row.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#errors
func TestACallbackThatPanicsStopsTheCallbacksAfterIt(t *testing.T) {
	t.Run("a check the caller started", func(t *testing.T) {
		svc, out := loggedService(nil)
		defer svc.Close()
		if err := svc.Register(&fakeEngine{}, "Eng", Config{
			Identifier: "a", DataFilePath: writeFile(t, "DATA")}); err != nil {
			t.Fatal(err)
		}
		var calls []string
		svc.OnUpdate("a", func(Complete) {
			calls = append(calls, "first")
			panic("callback test panic")
		})
		svc.OnUpdate("a", func(Complete) { calls = append(calls, "second") })

		err := svc.CheckForUpdate("a")
		if err == nil || !strings.Contains(err.Error(), "callback test panic") {
			t.Fatalf("CheckForUpdate gave %v, want the callback's panic", err)
		}
		if got := strings.Join(calls, ","); got != "first" {
			t.Errorf("the callbacks called were %q, want only the first", got)
		}
		if strings.Contains(out.String(), "level=ERROR") {
			t.Errorf("a check the caller started logged the error:\n%s", out)
		}
	})
	t.Run("a check update polling started", func(t *testing.T) {
		srv := httptest.NewServer(&requestRecorder{status: http.StatusNotModified})
		defer srv.Close()
		svc, out := loggedService(srv.Client())
		defer svc.Close()
		if err := svc.Register(&fakeEngine{}, "Eng", Config{
			Identifier: "a", DataUpdateURL: srv.URL,
			AutomaticUpdatesEnabled: true, PollingInterval: 10 * time.Millisecond,
		}); err != nil {
			t.Fatal(err)
		}
		svc.OnUpdate("a", func(Complete) { panic("callback test panic") })

		want := `level=ERROR msg="An unhandled error occurred while checking for automatic updates for engine 'Eng'" ` +
			`error="panic in an update callback: callback test panic"`
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(out.String(), want) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	})
}

type failingEngine struct{}

func (failingEngine) RefreshFromBytes([]byte) error { return errors.New("boom") }
func (failingEngine) RefreshFromFile(string) error  { return errors.New("boom") }
