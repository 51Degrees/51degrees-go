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
	"sync"
	"testing"
)

// recordingServer answers every request with the status and body, and records
// the If-Modified-Since header of each request.
func recordingServer(status int, body []byte) (*httptest.Server, func() []string) {
	var mu sync.Mutex
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("If-Modified-Since"))
		mu.Unlock()
		w.WriteHeader(status)
		w.Write(body)
	}))
	return server, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}

// With update on start-up on, the engine downloads a data file that is not
// there yet and then loads it.
func TestUpdateOnStartDownloadsMissingDataFile(t *testing.T) {
	good := readGoodFile(t)
	server, seen := recordingServer(http.StatusOK, good)
	defer server.Close()
	path := filepath.Join(t.TempDir(), "data.hash")

	e := newReloadTestEngine(t,
		WithDataFile(path),
		WithDataUpdateUrl(server.URL),
		WithUpdateOnStart(true),
		WithAutoUpdate(false),
		WithFileWatch(false))

	if _, err := os.Stat(path); err != nil {
		t.Errorf("the data file was not downloaded: %v", err)
	}
	if e.published().IsZero() {
		t.Error("the downloaded data file was not loaded")
	}
	if got := seen(); len(got) != 1 || got[0] != "" {
		t.Errorf("If-Modified-Since = %v, want none for a missing data file", got)
	}
}

// The engine fails when the start-up download fails and there is no data file
// to fall back to.
func TestUpdateOnStartFailsWithoutDataFile(t *testing.T) {
	server, _ := recordingServer(http.StatusInternalServerError, nil)
	defer server.Close()

	e, err := New(
		WithDataFile(filepath.Join(t.TempDir(), "data.hash")),
		WithDataUpdateUrl(server.URL),
		WithUpdateOnStart(true),
		WithAutoUpdate(false),
		WithFileWatch(false))
	if err == nil {
		e.Stop()
		t.Fatal("New succeeded with no data file and a failed download")
	}
	if !strings.Contains(err.Error(), "no local data file") {
		t.Errorf("error %q does not say there is no local data file", err)
	}
}

func TestUpdateOnStartKeepsDataFileWhenDownloadFails(t *testing.T) {
	good := readGoodFile(t)
	server, _ := recordingServer(http.StatusInternalServerError, nil)
	defer server.Close()

	e := newReloadTestEngine(t,
		WithDataFile(writeDataFile(t, good)),
		WithDataUpdateUrl(server.URL),
		WithUpdateOnStart(true),
		WithAutoUpdate(false),
		WithFileWatch(false))

	if e.published().IsZero() {
		t.Error("the existing data file was not loaded")
	}
}

// Without update on start-up a missing data file is still an error.
func TestMissingDataFileFailsWithoutUpdateOnStart(t *testing.T) {
	e, err := New(
		WithDataFile(filepath.Join(t.TempDir(), "data.hash")),
		WithAutoUpdate(false),
		WithFileWatch(false))
	if err == nil {
		e.Stop()
		t.Fatal("New succeeded with no data file")
	}
	if !strings.Contains(err.Error(), "failed to get file path") {
		t.Errorf("error %q does not say the file path failed", err)
	}
}

// The start-up update asks for data newer than the published date of the data
// file, not its file system date.
func TestUpdateOnStartSendsPublishedDate(t *testing.T) {
	good := readGoodFile(t)
	path := writeDataFile(t, good)
	first := newReloadTestEngine(t,
		WithDataFile(path),
		WithAutoUpdate(false),
		WithFileWatch(false))
	published := first.published()
	first.stop(t)

	server, seen := recordingServer(http.StatusNotModified, nil)
	defer server.Close()
	newReloadTestEngine(t,
		WithDataFile(path),
		WithDataUpdateUrl(server.URL),
		WithUpdateOnStart(true),
		WithAutoUpdate(false),
		WithFileWatch(false))

	got := seen()
	want := published.UTC().Format(http.TimeFormat)
	if len(got) != 1 || got[0] != want {
		t.Errorf("If-Modified-Since = %v, want the published date %q", got, want)
	}
}
