package common_go

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// cancelTestTimeoutSeconds is how long a stop may take while a pull is in
// progress.
const cancelTestTimeoutSeconds = 5

// slowDownload answers every request with the start of a data file and then
// holds the response open until the client goes or release is closed, as a
// large download does. started is closed when the first request arrives.
func slowDownload() (server *httptest.Server, started chan struct{}, release chan struct{}) {
	started = make(chan struct{})
	release = make(chan struct{})
	var once sync.Once
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		once.Do(func() { close(started) })
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	return server, started, release
}

// A stop that arrives during a download ends the download, so that stopping
// the puller does not wait for a large data file to finish downloading.
func TestScheduleFilePulling_StopEndsADownloadInProgress(t *testing.T) {
	server, started, release := slowDownload()
	defer server.Close()
	defer close(release)

	dataFile := filepath.Join(t.TempDir(), "data.hash")
	if err := os.WriteFile(dataFile, []byte("current"), 0644); err != nil {
		t.Fatal(err)
	}
	updater := NewFileUpdater(server.URL)
	updater.SetDataFile(dataFile)
	updater.SetIsFileWatcherEnabled(false)
	updater.SetUpdateOnStartEnabled(true)
	updater.SetDataFilePullEveryMs(60000)
	updater.SetRandomization(0)
	updater.SetLoggerEnabled(false)

	stopCh := make(chan *sync.WaitGroup)
	reload := make(chan struct{})
	go updater.ScheduleFilePulling(stopCh, reload)
	select {
	case <-started:
	case <-time.After(cancelTestTimeoutSeconds * time.Second):
		t.Fatal("the download did not start")
	}

	stopped := make(chan struct{})
	go func() {
		stopPuller(stopCh, reload)
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(cancelTestTimeoutSeconds * time.Second):
		t.Fatalf("stopping the puller waited more than %d seconds for the download",
			cancelTestTimeoutSeconds)
	}
}
