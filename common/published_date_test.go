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

// These tests follow the rule that a downloaded data file is loaded only when
// the published date in its header is later than that of the data loaded.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/data-updates.md#newfileavailable-via-remote-or-file-watcher

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// publishedDateLayout is how a test data file holds its published date, in
// place of the header of a real data file.
const publishedDateLayout = "2006-01-02"

// headerDate reads the published date a test data file holds, as an engine
// reads it from the header of a real one.
func headerDate(path string) (time.Time, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(publishedDateLayout, strings.TrimSpace(string(data)))
}

// fileServer answers every request with body.
func fileServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(body))
	}))
}

// newDatedUpdater returns an updater whose loaded data was published on
// loaded, with a data file that says so, and which reads the date of a new
// file with headerDate.
func newDatedUpdater(t *testing.T, url string, loaded time.Time) (*FileUpdater, string) {
	t.Helper()
	updater, _, dataFile := newRecordingUpdater(t, url)
	if err := os.WriteFile(dataFile, []byte(loaded.Format(publishedDateLayout)), 0644); err != nil {
		t.Fatal(err)
	}
	updater.SetPublishedDateFunc(func() time.Time { return loaded })
	updater.SetFilePublishedDateFunc(headerDate)
	return updater, dataFile
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A downloaded file published after the loaded data replaces the data file.
func TestPull_LoadsAFileWithALaterPublishedDate(t *testing.T) {
	loaded := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	later := loaded.AddDate(0, 0, 1).Format(publishedDateLayout)
	server := fileServer(later)
	defer server.Close()
	updater, dataFile := newDatedUpdater(t, server.URL, loaded)

	_, downloaded, err := updater.pull(nil)

	if err != nil || !downloaded {
		t.Fatalf("pull gave downloaded %v and error %v, want a download", downloaded, err)
	}
	if got := readText(t, dataFile); got != later {
		t.Errorf("the data file holds %q, want the downloaded %q", got, later)
	}
}

// A downloaded file published on or before the date of the loaded data is
// not loaded, however new the file is, and the check is not needed.
func TestPull_DoesNotLoadAFileThatIsNotPublishedLater(t *testing.T) {
	loaded := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	for name, published := range map[string]time.Time{
		"same date":    loaded,
		"earlier date": loaded.AddDate(0, 0, -1),
	} {
		t.Run(name, func(t *testing.T) {
			server := fileServer(published.Format(publishedDateLayout))
			defer server.Close()
			updater, dataFile := newDatedUpdater(t, server.URL, loaded)
			reloads := make(chan struct{}, 1)

			_, downloaded, err := updater.pull(reloads)

			if downloaded {
				t.Error("pull downloaded a file that is not published later")
			}
			if !errors.Is(err, ErrDataNotNewer) {
				t.Errorf("pull gave error %v, want %v", err, ErrDataNotNewer)
			}
			if status := pullUpdateStatus(err); status != UpdateNotNeeded {
				t.Errorf("the check is reported as %s, want %s", status, UpdateNotNeeded)
			}
			if got, want := readText(t, dataFile), loaded.Format(publishedDateLayout); got != want {
				t.Errorf("the data file holds %q, want the loaded %q", got, want)
			}
			if len(reloads) != 0 {
				t.Error("the engine was asked to reload")
			}
			assertNoPartFile(t, dataFile)
		})
	}
}

// A downloaded file whose published date cannot be read is loaded, so the
// engine's failure to load it is reported.
func TestPull_LoadsAFileWhosePublishedDateCannotBeRead(t *testing.T) {
	loaded := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	server := fileServer("Bad data")
	defer server.Close()
	updater, dataFile := newDatedUpdater(t, server.URL, loaded)

	_, downloaded, err := updater.pull(nil)

	if err != nil || !downloaded {
		t.Fatalf("pull gave downloaded %v and error %v, want a download", downloaded, err)
	}
	if got := readText(t, dataFile); got != "Bad data" {
		t.Errorf("the data file holds %q, want the downloaded file", got)
	}
}

// PublishedAfterLoaded decides from the dates alone, and says when it cannot.
func TestPublishedAfterLoaded(t *testing.T) {
	loaded := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	write := func(name, text string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	later := write("later", "2026-10-16")
	same := write("same", "2026-10-15")
	bad := write("bad", "Bad data")

	updater := NewFileUpdater("")
	if _, known := updater.PublishedAfterLoaded(later); known {
		t.Error("the date was known with no way to read it")
	}
	updater.SetFilePublishedDateFunc(headerDate)
	if _, known := updater.PublishedAfterLoaded(later); known {
		t.Error("the date was known with no loaded data")
	}
	updater.SetPublishedDateFunc(func() time.Time { return loaded })
	for _, c := range []struct {
		path         string
		newer, known bool
	}{
		{later, true, true},
		{same, false, true},
		{bad, false, false},
	} {
		newer, known := updater.PublishedAfterLoaded(c.path)
		if newer != c.newer || known != c.known {
			t.Errorf("%s gave newer %v and known %v, want %v and %v",
				c.path, newer, known, c.newer, c.known)
		}
	}
}
