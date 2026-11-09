package dataupdate

import (
	"errors"
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
			UpdateOnStartup: true, FileSystemWatcherEnabled: true,
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
	t.Run("errors", func(t *testing.T) {
		srv := httptest.NewServer(&requestRecorder{status: http.StatusInternalServerError})
		url := srv.URL
		defer srv.Close()
		svc, out := loggedService(srv.Client())
		defer svc.Close()
		if err := svc.Register(&fakeEngine{}, "Eng", Config{Identifier: "a", DataUpdateURL: url}); err != nil {
			t.Fatal(err)
		}
		_ = svc.CheckForUpdate("a")
		// A check made by the caller does not promise another attempt.
		want := "level=WARN msg=\"An error occurred while downloading a data file update for Eng from " +
			url + ". Error detail: "
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
		if strings.Contains(out.String(), "attempted again later") {
			t.Errorf("a check by the caller promised another attempt:\n%s", out)
		}
	})
	t.Run("automatic errors say the update will be tried again", func(t *testing.T) {
		srv := httptest.NewServer(&requestRecorder{status: http.StatusInternalServerError})
		defer srv.Close()
		svc, out := loggedService(srv.Client())
		defer svc.Close()
		if err := svc.Register(&fakeEngine{}, "Eng", Config{
			Identifier: "a", DataUpdateURL: srv.URL, AutomaticUpdatesEnabled: true,
			PollingInterval: 10 * time.Millisecond,
		}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(300 * time.Millisecond)
		want := "level=WARN msg=\"An error occurred while downloading a data file update for Eng from " +
			srv.URL + ". Update will be attempted again later. Error detail: "
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	})
	t.Run("integrity check", func(t *testing.T) {
		srv := httptest.NewServer(&requestRecorder{status: http.StatusOK, body: "DATA",
			header: http.Header{"Content-Md5": {"wrong"}}})
		defer srv.Close()
		svc, out := loggedService(srv.Client())
		defer svc.Close()
		if err := svc.Register(&fakeEngine{}, "Eng", Config{Identifier: "a", DataUpdateURL: srv.URL}); err != nil {
			t.Fatal(err)
		}
		_ = svc.CheckForUpdate("a")
		want := "level=WARN msg=\"An error occurred during the integrity check of new data file for Eng. Error detail: "
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	})
	t.Run("applying the update", func(t *testing.T) {
		svc, out := loggedService(nil)
		defer svc.Close()
		if err := svc.Register(&failingEngine{}, "Eng", Config{Identifier: "a"}); err != nil {
			t.Fatal(err)
		}
		_ = svc.UpdateFromMemory("a", []byte("x"))
		want := "level=ERROR msg=\"An error occurred while applying a data file update to Eng. Error detail: boom\""
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	})
}

type failingEngine struct{}

func (failingEngine) RefreshFromBytes([]byte) error { return errors.New("boom") }
func (failingEngine) RefreshFromFile(string) error  { return errors.New("boom") }
