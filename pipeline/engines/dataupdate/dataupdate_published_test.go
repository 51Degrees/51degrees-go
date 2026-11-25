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

// These tests follow the rule that a watched or downloaded data file is
// loaded only when the published date in its header is later than that of the
// data the Engine has loaded.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/data-updates.md#newfileavailable-via-remote-or-file-watcher

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

// publishedLayout is how a test data file holds its published date, in place
// of the header of a real data file.
const publishedLayout = "2006-01-02"

// loadedDate is the published date of the data each test starts with.
var loadedDate = time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)

// datedEngine reads the published date of a data file from the file, as an
// on-premise Engine reads it from the header, and remembers that of the data
// it loaded last.
type datedEngine struct {
	fakeEngine
	mu        sync.Mutex
	published time.Time
}

func newDatedEngine() *datedEngine { return &datedEngine{published: loadedDate} }

func (d *datedEngine) PublishedDateFromFile(path string) (time.Time, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(publishedLayout, strings.TrimSpace(string(data)))
}

func (d *datedEngine) RefreshFromFile(path string) error {
	published, err := d.PublishedDateFromFile(path)
	if err != nil {
		return err
	}
	if err := d.fakeEngine.RefreshFromFile(path); err != nil {
		return err
	}
	d.mu.Lock()
	d.published = published
	d.mu.Unlock()
	return nil
}

func (d *datedEngine) loaded() time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.published
}

func (d *datedEngine) refreshes() int {
	d.fakeEngine.mu.Lock()
	defer d.fakeEngine.mu.Unlock()
	return len(d.fromFiles)
}

func dated(t time.Time) string { return t.Format(publishedLayout) }

// waitForReport waits for the first report, and fails the test when none comes.
func waitForReport(t *testing.T, got *reports) Complete {
	t.Helper()
	select {
	case <-got.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the check was not reported")
	}
	return got.list()[0]
}

// registerDated registers a datedEngine for a data file holding the loaded
// date, and returns the reports of its checks.
func registerDated(t *testing.T, eng *datedEngine, cfg Config) (*Service, *reports) {
	t.Helper()
	svc := New(nil, nil)
	t.Cleanup(func() { svc.Close() })
	got := newReports()
	cfg.Identifier = "a"
	cfg.PublishedDate = eng.loaded
	cfg.OnComplete = got.record
	if err := svc.Register(eng, "Eng", cfg); err != nil {
		t.Fatal(err)
	}
	return svc, got
}

// A watched file published after the loaded data is loaded.
func TestAWatchedFileWithALaterPublishedDateIsLoaded(t *testing.T) {
	path := writeFile(t, dated(loadedDate))
	eng := newDatedEngine()
	_, got := registerDated(t, eng, Config{DataFilePath: path, FileSystemWatcherEnabled: true})

	later := loadedDate.AddDate(0, 0, 1)
	replace(t, path, dated(later), time.Now().Add(time.Hour))

	if report := waitForReport(t, got); report.Status != StatusSuccess {
		t.Errorf("the check was reported as %q, want %q", report.Status, StatusSuccess)
	}
	if !eng.loaded().Equal(later) {
		t.Errorf("the Engine holds data published %v, want %v", eng.loaded(), later)
	}
}

// A watched file published on or before the date of the loaded data is not
// loaded, even when its modification time is newer, and the check is not
// needed.
func TestAWatchedFileNotPublishedLaterIsNotLoaded(t *testing.T) {
	for name, published := range map[string]time.Time{
		"same date":    loadedDate,
		"earlier date": loadedDate.AddDate(0, 0, -1),
	} {
		t.Run(name, func(t *testing.T) {
			path := writeFile(t, dated(loadedDate))
			eng := newDatedEngine()
			_, got := registerDated(t, eng, Config{DataFilePath: path, FileSystemWatcherEnabled: true})

			replace(t, path, dated(published), time.Now().Add(time.Hour))

			if report := waitForReport(t, got); report.Status != StatusNotNeeded {
				t.Errorf("the check was reported as %q, want %q", report.Status, StatusNotNeeded)
			}
			if n := eng.refreshes(); n != 0 {
				t.Errorf("the Engine was refreshed %d times, want none", n)
			}
		})
	}
}

// A downloaded file published after the loaded data replaces the data file
// and is loaded.
func TestADownloadedFileWithALaterPublishedDateIsLoaded(t *testing.T) {
	later := loadedDate.AddDate(0, 0, 1)
	srv := httptest.NewServer(&requestRecorder{status: http.StatusOK, body: dated(later)})
	defer srv.Close()
	path := writeFile(t, dated(loadedDate))
	eng := newDatedEngine()
	svc, got := registerDated(t, eng, Config{DataFilePath: path, DataUpdateURL: srv.URL})

	if err := svc.CheckForUpdate("a"); err != nil {
		t.Fatal(err)
	}

	if report := waitForReport(t, got); report.Status != StatusSuccess {
		t.Errorf("the check was reported as %q, want %q", report.Status, StatusSuccess)
	}
	if !eng.loaded().Equal(later) {
		t.Errorf("the Engine holds data published %v, want %v", eng.loaded(), later)
	}
	if content, _ := os.ReadFile(path); string(content) != dated(later) {
		t.Errorf("the data file holds %q, want %q", content, dated(later))
	}
}

// A downloaded file published on or before the date of the loaded data
// leaves the data file and the Engine as they are, and the check is not
// needed.
func TestADownloadedFileNotPublishedLaterIsNotLoaded(t *testing.T) {
	for name, published := range map[string]time.Time{
		"same date":    loadedDate,
		"earlier date": loadedDate.AddDate(0, 0, -1),
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(&requestRecorder{status: http.StatusOK, body: dated(published)})
			defer srv.Close()
			path := writeFile(t, dated(loadedDate))
			eng := newDatedEngine()
			svc, got := registerDated(t, eng, Config{DataFilePath: path, DataUpdateURL: srv.URL})

			if err := svc.CheckForUpdate("a"); err != nil {
				t.Fatal(err)
			}

			if report := waitForReport(t, got); report.Status != StatusNotNeeded {
				t.Errorf("the check was reported as %q, want %q", report.Status, StatusNotNeeded)
			}
			if n := eng.refreshes(); n != 0 {
				t.Errorf("the Engine was refreshed %d times, want none", n)
			}
			if content, _ := os.ReadFile(path); string(content) != dated(loadedDate) {
				t.Errorf("the data file holds %q, want %q", content, dated(loadedDate))
			}
			left, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp-*"))
			if len(left) != 0 {
				t.Errorf("the download left %v", left)
			}
		})
	}
}
