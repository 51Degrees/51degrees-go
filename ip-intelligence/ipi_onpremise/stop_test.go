package ipi_onpremise

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Stop does not wait for a download in progress, which for a large data file
// can take many minutes.
func TestStopDoesNotWaitForADownload(t *testing.T) {
	const startTimeoutSeconds = 10
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		once.Do(func() { close(started) })
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)

	e := newReloadTestEngine(t,
		WithDataFile(writeDataFile(t, readGoodFile(t))),
		WithDataUpdateUrl(server.URL),
		WithPollingInterval(1),
		WithRandomization(0),
		WithFileWatch(false))
	select {
	case <-started:
	case <-time.After(startTimeoutSeconds * time.Second):
		t.Fatal("the download did not start")
	}
	e.stop(t)
}
