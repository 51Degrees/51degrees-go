package common_go

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordedLog is a LogWriter that records the messages of each level. It has
// levels, so the wrapper writes through them.
type recordedLog struct {
	mu      sync.Mutex
	entries []string
}

func (r *recordedLog) add(level, format string, v ...interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, level+" "+sprintf(format, v...))
}

func (r *recordedLog) Printf(format string, v ...interface{}) { r.add("PRINT", format, v...) }
func (r *recordedLog) Infof(format string, v ...interface{})  { r.add("INFO", format, v...) }
func (r *recordedLog) Warnf(format string, v ...interface{})  { r.add("WARN", format, v...) }
func (r *recordedLog) Errorf(format string, v ...interface{}) { r.add("ERROR", format, v...) }

func (r *recordedLog) has(entry string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.entries {
		if e == entry {
			return true
		}
	}
	return false
}

func (r *recordedLog) hasPrefix(prefix string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.entries {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}

func (r *recordedLog) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.entries, "\n")
}

func sprintf(format string, v ...interface{}) string {
	return fmt.Sprintf(format, v...)
}

// newRecordingUpdater returns an updater for a data file in a new directory
// that writes its messages to the returned log.
func newRecordingUpdater(t *testing.T, url string) (*FileUpdater, *recordedLog, string) {
	t.Helper()
	dataFile := filepath.Join(t.TempDir(), "data.hash")
	updater := NewFileUpdater(url)
	updater.SetDataFile(dataFile)
	updater.SetEngineName("TestEngine")
	updater.SetIsFileWatcherEnabled(false)
	log := &recordedLog{}
	updater.SetCustomLogger(log)
	return updater, log, dataFile
}

// headerServer records the If-Modified-Since header of each request and
// answers with the given status and body.
func headerServer(status int, body string) (*httptest.Server, func() []string) {
	var mu sync.Mutex
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("If-Modified-Since"))
		mu.Unlock()
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	return server, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}

// The specification asks for If-Modified-Since to be the publication date of
// the data file, with the file system date only as the fall back.
func TestPull_SendsPublishedDate(t *testing.T) {
	server, seen := headerServer(http.StatusNotModified, "")
	defer server.Close()
	updater, _, dataFile := newRecordingUpdater(t, server.URL)
	if err := os.WriteFile(dataFile, []byte("current"), 0644); err != nil {
		t.Fatal(err)
	}
	modTime := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(dataFile, modTime, modTime); err != nil {
		t.Fatal(err)
	}
	published := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	updater.SetPublishedDateFunc(func() time.Time { return published })

	updater.doFilePulling(make(chan struct{}, 1))

	got := seen()
	if len(got) != 1 || got[0] != published.Format(http.TimeFormat) {
		t.Errorf("If-Modified-Since = %v, want the published date %q", got, published.Format(http.TimeFormat))
	}
}

func TestPull_FallsBackToModificationTime(t *testing.T) {
	modTime := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	cases := map[string]func() time.Time{
		"no function":        nil,
		"zero date":          func() time.Time { return time.Time{} },
		"year 0 in the file": func() time.Time { return time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC) },
	}
	for name, published := range cases {
		t.Run(name, func(t *testing.T) {
			server, seen := headerServer(http.StatusNotModified, "")
			defer server.Close()
			updater, _, dataFile := newRecordingUpdater(t, server.URL)
			if err := os.WriteFile(dataFile, []byte("current"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(dataFile, modTime, modTime); err != nil {
				t.Fatal(err)
			}
			if published != nil {
				updater.SetPublishedDateFunc(published)
			}

			updater.doFilePulling(make(chan struct{}, 1))

			got := seen()
			if len(got) != 1 || got[0] != modTime.Format(http.TimeFormat) {
				t.Errorf("If-Modified-Since = %v, want the modification time %q", got, modTime.Format(http.TimeFormat))
			}
		})
	}
}

// A missing data file is not an error for the pull, which downloads it.
func TestPull_DownloadsMissingDataFile(t *testing.T) {
	server, seen := headerServer(http.StatusOK, "new data")
	defer server.Close()
	updater, _, dataFile := newRecordingUpdater(t, server.URL)

	if next := updater.doFilePulling(make(chan struct{}, 1)); next != updater.GetDataFilePullEveryMs() {
		t.Errorf("next pull in %d ms, want %d", next, updater.GetDataFilePullEveryMs())
	}
	if got := seen(); len(got) != 1 || got[0] != "" {
		t.Errorf("If-Modified-Since = %v, want none for a missing data file", got)
	}
	if got := readTempFile(t, dataFile); got != "new data" {
		t.Errorf("data file content = %q, want the download", got)
	}
}

func TestCheckDataFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.hash")
	updater := NewFileUpdater("http://localhost")
	updater.SetDataFile(missing)
	if err := updater.CheckDataFile(); err == nil || !strings.Contains(err.Error(), "failed to get file path") {
		t.Errorf("missing file, update on start off: error = %v, want failed to get file path", err)
	}
	updater.SetUpdateOnStartEnabled(true)
	if err := updater.CheckDataFile(); err != nil {
		t.Errorf("missing file, update on start on: error = %v, want none", err)
	}
}

func TestPullOnStart(t *testing.T) {
	t.Run("downloads the missing data file", func(t *testing.T) {
		server, _ := headerServer(http.StatusOK, "new data")
		defer server.Close()
		updater, _, dataFile := newRecordingUpdater(t, server.URL)
		downloaded, err := updater.PullOnStart()
		if err != nil || !downloaded {
			t.Fatalf("PullOnStart() = %v, %v, want true and no error", downloaded, err)
		}
		if got := readTempFile(t, dataFile); got != "new data" {
			t.Errorf("data file content = %q, want the download", got)
		}
	})
	t.Run("fails when the download fails and there is no data file", func(t *testing.T) {
		server, _ := headerServer(http.StatusInternalServerError, "")
		defer server.Close()
		updater, _, _ := newRecordingUpdater(t, server.URL)
		if _, err := updater.PullOnStart(); err == nil {
			t.Error("PullOnStart() = nil, want an error")
		}
	})
	t.Run("keeps the data file when the download fails", func(t *testing.T) {
		server, _ := headerServer(http.StatusInternalServerError, "")
		defer server.Close()
		updater, _, dataFile := newRecordingUpdater(t, server.URL)
		if err := os.WriteFile(dataFile, []byte("current"), 0644); err != nil {
			t.Fatal(err)
		}
		if downloaded, err := updater.PullOnStart(); err != nil || downloaded {
			t.Errorf("PullOnStart() = %v, %v, want false and no error", downloaded, err)
		}
		if got := readTempFile(t, dataFile); got != "current" {
			t.Errorf("data file content = %q, want unchanged", got)
		}
	})
	t.Run("a current data file is not an error", func(t *testing.T) {
		server, _ := headerServer(http.StatusNotModified, "")
		defer server.Close()
		updater, _, dataFile := newRecordingUpdater(t, server.URL)
		if err := os.WriteFile(dataFile, []byte("current"), 0644); err != nil {
			t.Fatal(err)
		}
		if downloaded, err := updater.PullOnStart(); err != nil || downloaded {
			t.Errorf("PullOnStart() = %v, %v, want false and no error", downloaded, err)
		}
	})
	t.Run("does not announce the file", func(t *testing.T) {
		server, _ := headerServer(http.StatusOK, "new data")
		defer server.Close()
		updater, _, _ := newRecordingUpdater(t, server.URL)
		// The channel has no reader, so an announcement would block.
		done := make(chan error, 1)
		go func() {
			_, err := updater.PullOnStart()
			done <- err
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("PullOnStart() blocked announcing the file")
		}
	})
}

func TestScheduleFilePulling_SkipsStartUpPullAfterPullOnStart(t *testing.T) {
	server, seen := headerServer(http.StatusOK, "new data")
	defer server.Close()
	updater, _, _ := newRecordingUpdater(t, server.URL)
	updater.SetUpdateOnStartEnabled(true)
	updater.SetDataFilePullEveryMs(60 * 60 * 1000)
	updater.SetRandomization(0)
	if _, err := updater.PullOnStart(); err != nil {
		t.Fatal(err)
	}
	stopCh := make(chan *sync.WaitGroup)
	reload := make(chan struct{}, 1)
	go updater.ScheduleFilePulling(stopCh, reload)
	stopPuller(stopCh, reload)

	if got := seen(); len(got) != 1 {
		t.Errorf("server was asked %d times, want once", len(got))
	}
}

// Polling does not start before the date the data file expects an update.
func TestDelayBeforeFirstPull(t *testing.T) {
	const everyMs = 30 * 60 * 1000
	newUpdater := func(next func() time.Time) *FileUpdater {
		updater := NewFileUpdater("http://localhost")
		updater.SetDataFilePullEveryMs(everyMs)
		if next != nil {
			updater.SetNextUpdateFunc(next)
		}
		return updater
	}
	approx := func(t *testing.T, got, want int) {
		t.Helper()
		if got > want || got < want-5000 {
			t.Errorf("delay = %d ms, want just under %d ms", got, want)
		}
	}

	t.Run("a later date replaces the interval", func(t *testing.T) {
		in := 3 * time.Hour
		updater := newUpdater(func() time.Time { return time.Now().Add(in) })
		approx(t, updater.delayBeforeFirstPull(everyMs), int(in/time.Millisecond))
	})
	t.Run("a nearer date replaces the interval", func(t *testing.T) {
		in := 5 * time.Minute
		updater := newUpdater(func() time.Time { return time.Now().Add(in) })
		approx(t, updater.delayBeforeFirstPull(everyMs), int(in/time.Millisecond))
	})
	t.Run("a wait asked for by the server is not shortened", func(t *testing.T) {
		updater := newUpdater(func() time.Time { return time.Now().Add(5 * time.Minute) })
		if got := updater.delayBeforeFirstPull(3600 * 1000); got != 3600*1000 {
			t.Errorf("delay = %d ms, want 3600000", got)
		}
	})
	t.Run("a past date leaves the interval", func(t *testing.T) {
		updater := newUpdater(func() time.Time { return time.Now().Add(-time.Hour) })
		if got := updater.delayBeforeFirstPull(everyMs); got != everyMs {
			t.Errorf("delay = %d ms, want %d", got, everyMs)
		}
	})
	t.Run("a data file with no date leaves the interval", func(t *testing.T) {
		updater := newUpdater(func() time.Time { return time.Time{} })
		if got := updater.delayBeforeFirstPull(everyMs); got != everyMs {
			t.Errorf("delay = %d ms, want %d", got, everyMs)
		}
	})
	t.Run("no function leaves the interval", func(t *testing.T) {
		if got := newUpdater(nil).delayBeforeFirstPull(everyMs); got != everyMs {
			t.Errorf("delay = %d ms, want %d", got, everyMs)
		}
	})
}

// With an update expected in the future the scheduler makes no request before
// then, even when the interval is short.
func TestScheduleFilePulling_WaitsForNextUpdate(t *testing.T) {
	server, seen := headerServer(http.StatusNotModified, "")
	defer server.Close()
	updater, _, dataFile := newRecordingUpdater(t, server.URL)
	if err := os.WriteFile(dataFile, []byte("current"), 0644); err != nil {
		t.Fatal(err)
	}
	updater.SetDataFilePullEveryMs(10)
	updater.SetRandomization(0)
	updater.SetNextUpdateFunc(func() time.Time { return time.Now().Add(time.Hour) })

	stopCh := make(chan *sync.WaitGroup)
	reload := make(chan struct{}, 1)
	go updater.ScheduleFilePulling(stopCh, reload)
	time.Sleep(300 * time.Millisecond)
	stopPuller(stopCh, reload)

	if got := seen(); len(got) != 0 {
		t.Errorf("server was asked %d times before the expected update, want none", len(got))
	}
}

func TestPull_LogsInformation(t *testing.T) {
	t.Run("a download", func(t *testing.T) {
		server, _ := headerServer(http.StatusOK, "new data")
		defer server.Close()
		updater, log, _ := newRecordingUpdater(t, server.URL)
		updater.doFilePulling(make(chan struct{}, 1))
		for _, want := range []string{
			"INFO Checking for update",
			"INFO Checking for update from '" + server.URL + "' for Engine 'TestEngine'",
			"INFO Downloaded new data from '" + server.URL + "' for Engine 'TestEngine'",
		} {
			if !log.has(want) {
				t.Errorf("missing log entry %q in:\n%s", want, log)
			}
		}
	})
	t.Run("no newer data", func(t *testing.T) {
		server, _ := headerServer(http.StatusNotModified, "")
		defer server.Close()
		updater, log, dataFile := newRecordingUpdater(t, server.URL)
		if err := os.WriteFile(dataFile, []byte("current"), 0644); err != nil {
			t.Fatal(err)
		}
		updater.doFilePulling(make(chan struct{}, 1))
		if !log.hasPrefix("INFO No data newer than ") ||
			!strings.Contains(log.String(), " found at '"+server.URL+"' for Engine 'TestEngine'") {
			t.Errorf("missing no newer data entry in:\n%s", log)
		}
	})
	t.Run("update on start-up", func(t *testing.T) {
		server, _ := headerServer(http.StatusOK, "new data")
		defer server.Close()
		updater, log, _ := newRecordingUpdater(t, server.URL)
		if _, err := updater.PullOnStart(); err != nil {
			t.Fatal(err)
		}
		if !log.has("INFO Updating on start-up") {
			t.Errorf("missing log entry in:\n%s", log)
		}
	})
}

// Each failed pull is logged as the error table gives it, which follows .NET.
// A failure with a row of its own is logged at Error as "Error during check
// for data update" and the row's message, and any other at Error with the
// last row's message.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#errors
func TestPull_LogsErrorsAtTheLevelsOfTheSpecification(t *testing.T) {
	const checkError = "ERROR Error during check for data update. "
	t.Run("cannot connect", func(t *testing.T) {
		server, _ := headerServer(http.StatusOK, "")
		url := server.URL
		server.Close()
		updater, log, _ := newRecordingUpdater(t, url)
		updater.doFilePulling(make(chan struct{}, 1))
		want := checkError + "Error accessing data update service at '" + url +
			"' for engine 'TestEngine'. Error detail: "
		if !log.hasPrefix(want) {
			t.Errorf("missing log entry starting %q in:\n%s", want, log)
		}
	})
	for _, test := range []struct {
		name   string
		status int
		want   string
	}{
		{"too many requests", http.StatusTooManyRequests, "Too many requests to '%s' for engine 'TestEngine'"},
		{"forbidden", http.StatusForbidden, "Access denied to data update service at '%s' for engine 'TestEngine'"},
		{"another status", http.StatusInternalServerError,
			"HTTP status code '500' from data update service at '%s' for engine 'TestEngine'"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server, _ := headerServer(test.status, "")
			defer server.Close()
			updater, log, _ := newRecordingUpdater(t, server.URL)
			updater.doFilePulling(make(chan struct{}, 1))
			want := checkError + fmt.Sprintf(test.want, server.URL)
			if !log.has(want) {
				t.Errorf("missing log entry %q in:\n%s", want, log)
			}
		})
	}
	t.Run("integrity check", func(t *testing.T) {
		server := createMockServer(http.StatusOK, map[string]string{"Content-MD5": "wrong"}, []byte("new"))
		defer server.Close()
		updater, log, _ := newRecordingUpdater(t, server.URL)
		updater.doFilePulling(make(chan struct{}, 1))
		want := checkError + "Integrity check failed. MD5 hash in HTTP response 'wrong' for " +
			"'TestEngine'data update does not match calculated hash for the downloaded file '"
		if !log.hasPrefix(want) {
			t.Errorf("missing log entry starting %q in:\n%s", want, log)
		}
	})
	t.Run("no MD5 hash", func(t *testing.T) {
		server, _ := headerServer(http.StatusOK, "new data")
		defer server.Close()
		updater, log, _ := newRecordingUpdater(t, server.URL)
		updater.doFilePulling(make(chan struct{}, 1))
		want := "WARN No MD5 hash included in data update response for '" + server.URL +
			"'. Unable to verify data integrity"
		if !log.has(want) {
			t.Errorf("missing log entry %q in:\n%s", want, log)
		}
	})
	t.Run("writing the download", func(t *testing.T) {
		server, _ := headerServer(http.StatusOK, "new data")
		defer server.Close()
		updater, log, _ := newRecordingUpdater(t, server.URL)
		// The directory does not exist, so no temp file can be made, which the
		// table has no row for.
		missingDir := filepath.Join(t.TempDir(), "missing", "data.hash")
		updater.SetDataFile(missingDir)
		updater.doFilePulling(make(chan struct{}, 1))
		want := "ERROR An unhandled error occurred while checking for automatic updates for engine " +
			"'TestEngine'. Error detail: "
		if !log.hasPrefix(want) {
			t.Errorf("missing log entry starting %q in:\n%s", want, log)
		}
	})
}

// A writer without levels receives the level at the start of the message.
func TestLogWrapper_LevelsWithoutLevelWriter(t *testing.T) {
	mock := &plainLog{}
	wrapper := &LogWrapper{enabled: true, logger: mock}
	wrapper.Infof("a %s", "b")
	wrapper.Warnf("c")
	wrapper.Errorf("d")
	want := []string{"INFO a b", "WARN c", "ERROR d"}
	if strings.Join(mock.entries, "|") != strings.Join(want, "|") {
		t.Errorf("entries = %v, want %v", mock.entries, want)
	}
	disabled := &LogWrapper{enabled: false, logger: mock}
	disabled.Infof("ignored")
	if len(mock.entries) != 3 {
		t.Errorf("a disabled wrapper wrote %v", mock.entries)
	}
}

type plainLog struct{ entries []string }

func (p *plainLog) Printf(format string, v ...interface{}) {
	p.entries = append(p.entries, sprintf(format, v...))
}

func TestPullError_KeepsTheTextAndTheCause(t *testing.T) {
	cause := errors.New("boom")
	err := newPullError(pullDownload, cause)
	if err.Error() != "boom" || !errors.Is(err, cause) {
		t.Errorf("error %q does not keep the text and cause", err)
	}
}

// A data file that cannot be replaced is reported in the words of the error
// table, which follows .NET, whether or not the Engine reads the file itself.
func TestPull_LogsAFileThatCannotBeReplaced(t *testing.T) {
	const want = "ERROR Error during check for data update. An error occurred when copying a data file to " +
		"replace the existing one at '"
	cases := map[string]struct {
		tempCopy bool
		want     string
	}{
		"use temp file on":  {true, want},
		"use temp file off": {false, want},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			server, _ := headerServer(http.StatusOK, "new data")
			defer server.Close()
			updater, log, dataFile := newRecordingUpdater(t, server.URL)
			updater.SetIsCreateTempDataCopyEnabled(c.tempCopy)
			// A directory with a file in it cannot be replaced by a rename.
			if err := os.MkdirAll(dataFile, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dataFile, "child"), []byte("x"), 0644); err != nil {
				t.Fatal(err)
			}
			updater.doFilePulling(make(chan struct{}, 1))
			if !log.hasPrefix(c.want) {
				t.Errorf("missing log entry starting %q in:\n%s", c.want, log)
			}
		})
	}
}

func TestPull_LogsAMissingTemporaryFilePath(t *testing.T) {
	updater := NewFileUpdater("http://localhost")
	updater.SetEngineName("TestEngine")
	log := &recordedLog{}
	updater.SetCustomLogger(log)
	if next := updater.doFilePulling(make(chan struct{}, 1)); next != retryMs {
		t.Errorf("next pull in %d ms, want %d", next, retryMs)
	}
	want := "ERROR Error during check for data update. The data file 'TestEngine' is checking for updates " +
		"but does not have a temporary file path configured."
	if !log.has(want) {
		t.Errorf("missing log entry %q in:\n%s", want, log)
	}
}
