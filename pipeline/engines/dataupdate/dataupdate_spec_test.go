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
	"sync/atomic"
	"testing"
	"time"
)

// The specification asks for If-Modified-Since to be the publication date of
// the data file, then the file system date as the fall back.
func TestIfModifiedSinceUsesPublishedDate(t *testing.T) {
	recorder := &requestRecorder{
		status: http.StatusNotModified,
		header: http.Header{"Last-Modified": {"Mon, 02 Jan 2006 15:04:05 GMT"}},
	}
	srv := httptest.NewServer(recorder)
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "data.bin")
	if err := os.WriteFile(path, []byte("current"), 0644); err != nil {
		t.Fatal(err)
	}
	modTime := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
	published := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	var publishedDate atomic.Value
	publishedDate.Store(published)

	svc := New(nil, srv.Client())
	defer svc.Close()
	if err := svc.Register(&fakeEngine{}, "published", Config{
		Identifier:         "published",
		DataFilePath:       path,
		DataUpdateURL:      srv.URL,
		UseIfModifiedSince: true,
		PublishedDate:      func() time.Time { return publishedDate.Load().(time.Time) },
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.CheckForUpdate("published"); err != nil {
		t.Fatal(err)
	}
	// A data file with no publication date falls back to the file's time.
	publishedDate.Store(time.Time{})
	if err := svc.CheckForUpdate("published"); err != nil {
		t.Fatal(err)
	}

	got := recorder.requests()
	want := []string{published.Format(http.TimeFormat), modTime.Format(http.TimeFormat)}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("If-Modified-Since = %v, want %v", got, want)
	}
}

// Polling does not start before the date the data expects an update.
func TestFirstPollWaitsForNextUpdate(t *testing.T) {
	cases := map[string]struct {
		next     func() time.Time
		wantPoll bool
	}{
		"date in the future": {func() time.Time { return time.Now().Add(time.Hour) }, false},
		"date in the past":   {func() time.Time { return time.Now().Add(-time.Hour) }, true},
		"no date":            {func() time.Time { return time.Time{} }, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			recorder := &requestRecorder{status: http.StatusNotModified}
			srv := httptest.NewServer(recorder)
			defer srv.Close()
			svc := New(nil, srv.Client())
			defer svc.Close()
			if err := svc.Register(&fakeEngine{}, "poll", Config{
				Identifier:              "poll",
				DataUpdateURL:           srv.URL,
				AutomaticUpdatesEnabled: true,
				PollingInterval:         10 * time.Millisecond,
				NextUpdate:              c.next,
			}); err != nil {
				t.Fatal(err)
			}
			time.Sleep(400 * time.Millisecond)
			if polled := len(recorder.requests()) > 0; polled != c.wantPoll {
				t.Errorf("polled = %v, want %v", polled, c.wantPoll)
			}
		})
	}
}

func TestFirstDelay(t *testing.T) {
	const regular = 30 * time.Minute
	svc := New(nil, nil)
	reg := &registration{svc: svc, cfg: Config{MaxRandomization: 10 * time.Minute}}

	reg.cfg.NextUpdate = func() time.Time { return time.Now().Add(2 * time.Hour) }
	got := reg.firstDelay(regular)
	if got < 2*time.Hour-time.Minute || got > 2*time.Hour+10*time.Minute {
		t.Errorf("delay = %v, want the time to the expected update plus up to the randomization", got)
	}

	reg.cfg.NextUpdate = func() time.Time { return time.Now().Add(-time.Hour) }
	if got := reg.firstDelay(regular); got != regular {
		t.Errorf("delay = %v, want %v for a date in the past", got, regular)
	}

	reg.cfg.NextUpdate = nil
	if got := reg.firstDelay(regular); got != regular {
		t.Errorf("delay = %v, want %v with no function", got, regular)
	}
}

// With no local data file a failed start-up download is an error, and with one
// the service falls back to it.
func TestUpdateOnStartupFailure(t *testing.T) {
	srv := httptest.NewServer(&requestRecorder{status: http.StatusInternalServerError})
	defer srv.Close()

	t.Run("no local data file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.bin")
		svc := New(nil, srv.Client())
		defer svc.Close()
		cfg := Config{
			Identifier:      "start",
			DataFilePath:    path,
			DataUpdateURL:   srv.URL,
			UpdateOnStartup: true,
		}
		if err := svc.Register(&fakeEngine{}, "start", cfg); err == nil {
			t.Fatal("Register() = nil, want an error")
		}
		// Nothing stays registered after the failure.
		if err := svc.CheckForUpdate("start"); err == nil {
			t.Error("a failed registration was kept")
		}
	})
	t.Run("local data file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "data.bin")
		if err := os.WriteFile(path, []byte("current"), 0644); err != nil {
			t.Fatal(err)
		}
		svc := New(nil, srv.Client())
		defer svc.Close()
		if err := svc.Register(&fakeEngine{}, "start", Config{
			Identifier:      "start",
			DataFilePath:    path,
			DataUpdateURL:   srv.URL,
			UpdateOnStartup: true,
		}); err != nil {
			t.Errorf("Register() = %v, want none", err)
		}
	})
}

func TestUpdateFromMemory(t *testing.T) {
	eng := &fakeEngine{}
	svc := New(nil, nil)
	defer svc.Close()
	if err := svc.Register(eng, "memory", Config{Identifier: "memory"}); err != nil {
		t.Fatal(err)
	}
	var called int32
	svc.OnUpdate("memory", func() { atomic.AddInt32(&called, 1) })

	if err := svc.UpdateFromMemory("memory", []byte("NEW DATA")); err != nil {
		t.Fatalf("UpdateFromMemory() = %v", err)
	}
	if eng.bytesCount() != 1 || string(eng.fromBytes[0]) != "NEW DATA" {
		t.Errorf("engine was refreshed with %q, want NEW DATA", eng.fromBytes)
	}
	if atomic.LoadInt32(&called) != 1 {
		t.Error("the update callback did not run")
	}
	if err := svc.UpdateFromMemory("unknown", []byte("x")); err == nil {
		t.Error("UpdateFromMemory() for an unknown identifier = nil, want an error")
	}
	if err := svc.UpdateFromMemory("memory", nil); err == nil {
		t.Error("UpdateFromMemory() with no data = nil, want an error")
	}
}
