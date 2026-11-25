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

package onpremise

// These tests follow the rule that a watched or downloaded data file is
// loaded only when the published date in its header is later than that of the
// data the engine has loaded.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/data-updates.md#newfileavailable-via-remote-or-file-watcher

import (
	"bytes"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	common_go "github.com/51Degrees/51degrees-go/common/v4"
)

// refreshLogged is logged each time the engine loads a data file again.
const refreshLogged = "Attempting to refresh Engine"

// datedGoodFile returns the good file, published on from, with the published
// date in its header changed to to. The header holds the published date as a
// little endian year, then the month and the day.
func datedGoodFile(t *testing.T, good []byte, from, to time.Time) []byte {
	t.Helper()
	date := func(d time.Time) []byte {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint16(b, uint16(d.Year()))
		b[2] = byte(d.Month())
		b[3] = byte(d.Day())
		return b
	}
	i := bytes.Index(good[:reloadTestHeaderBytes], date(from))
	if i < 0 {
		t.Fatalf("published date %v not found in the header", from)
	}
	dated := append([]byte{}, good...)
	copy(dated[i:], date(to))
	return dated
}

// notLaterDates gives a published date the same as, and one before, the
// published date given.
func notLaterDates(published time.Time) map[string]time.Time {
	return map[string]time.Time{
		"same date":    published,
		"earlier date": published.AddDate(0, 0, -1),
	}
}

// The engine's method reads the published date from the header alone.
func TestPublishedDateFromFile(t *testing.T) {
	good := readGoodFile(t)
	data := writeDataFile(t, good)
	e := newReloadTestEngine(t,
		WithDataFile(data),
		WithAutoUpdate(false),
		WithFileWatch(false))
	later := e.published().AddDate(0, 0, 1)
	path := writeDataFile(t, datedGoodFile(t, good, e.published(), later))

	got, err := e.PublishedDateFromFile(path)

	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(later) {
		t.Errorf("the header gives %v, want %v", got, later)
	}
	if _, err := e.PublishedDateFromFile(writeDataFile(t, []byte("Bad data"))); err == nil {
		t.Error("a file that is not a data file gave no error")
	}
}

// A watched file published after the loaded data is loaded.
func TestAWatchedFileWithALaterPublishedDateIsLoaded(t *testing.T) {
	good := readGoodFile(t)
	reports := &reportLog{}
	data := writeDataFile(t, good)
	e := newWatchedEngine(t, data, reports)
	published := e.published()
	later := published.AddDate(0, 0, 1)

	renameIn(t, data, datedGoodFile(t, good, published, later),
		time.Now().Add(time.Hour))

	waitFor(t, "the check to be reported", func() bool {
		return len(reports.statuses()) > 0
	})
	if got := reports.statuses(); len(got) != 1 || got[0] != common_go.UpdateSuccess {
		t.Errorf("the later file was reported as %v, want [%s]", got,
			common_go.UpdateSuccess)
	}
	if !e.published().Equal(later) {
		t.Errorf("the engine holds data published %v, want %v",
			e.published(), later)
	}
}

// A watched file published on or before the date of the loaded data is not
// loaded, even when its modification time is newer, and the check is
// reported as not needed.
func TestAWatchedFileNotPublishedLaterIsNotLoaded(t *testing.T) {
	good := readGoodFile(t)
	for name, date := range notLaterDates(readPublished(t, good)) {
		t.Run(name, func(t *testing.T) {
			reports := &reportLog{}
			logs := &reloadTestLog{}
			data := writeDataFile(t, good)
			e := newReloadTestEngine(t,
				WithDataFile(data),
				WithAutoUpdate(false),
				WithFileWatch(true),
				WithTempDataCopy(true),
				WithCustomLogger(logs),
				WithUpdateCallback(reports.add))
			published := e.published()

			renameIn(t, data, datedGoodFile(t, good, published, date),
				time.Now().Add(time.Hour))

			waitFor(t, "the check to be reported", func() bool {
				return len(reports.statuses()) > 0
			})
			if got := reports.statuses(); len(got) != 1 || got[0] != common_go.UpdateNotNeeded {
				t.Errorf("the file was reported as %v, want [%s]", got,
					common_go.UpdateNotNeeded)
			}
			if logs.has(refreshLogged) {
				t.Error("the engine loaded the file")
			}
			if !e.published().Equal(published) {
				t.Errorf("the engine holds data published %v, want %v",
					e.published(), published)
			}
		})
	}
}

// A downloaded file published after the loaded data is loaded.
func TestADownloadedFileWithALaterPublishedDateIsLoaded(t *testing.T) {
	good := readGoodFile(t)
	server := newUpdateServer()
	defer server.Close()
	reports := &reportLog{}
	e := newReloadTestEngine(t,
		WithDataFile(writeDataFile(t, good)),
		WithDataUpdateUrl(server.URL),
		WithPollingInterval(1),
		WithRandomization(0),
		WithFileWatch(false),
		WithCustomLogger(&reloadTestLog{}),
		WithUpdateCallback(reports.add))
	later := e.published().AddDate(0, 0, 1)

	server.publish(datedGoodFile(t, good, e.published(), later))

	waitFor(t, "the later file to be loaded", func() bool {
		return e.published().Equal(later)
	})
	waitFor(t, "the load to be reported", func() bool {
		return reports.count(common_go.UpdateSuccess) == 1
	})
}

// countingServer sends the same file for every request and counts them.
type countingServer struct {
	*httptest.Server
	requests atomic.Int32
}

func newCountingServer(file []byte) *countingServer {
	s := &countingServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(file)
			s.requests.Add(1)
		}))
	return s
}

// A downloaded file published on or before the date of the loaded data
// leaves the data file and the engine as they are, and the check is reported
// as not needed.
func TestADownloadedFileNotPublishedLaterIsNotLoaded(t *testing.T) {
	good := readGoodFile(t)
	for name, date := range notLaterDates(readPublished(t, good)) {
		t.Run(name, func(t *testing.T) {
			published := readPublished(t, good)
			server := newCountingServer(datedGoodFile(t, good, published, date))
			defer server.Close()
			reports := &reportLog{}
			logs := &reloadTestLog{}
			data := writeDataFile(t, good)
			e := newReloadTestEngine(t,
				WithDataFile(data),
				WithDataUpdateUrl(server.URL),
				WithPollingInterval(1),
				WithRandomization(0),
				WithFileWatch(false),
				WithCustomLogger(logs),
				WithUpdateCallback(reports.add))

			// A check is reported before the next one asks for the file.
			waitFor(t, "a check to end", func() bool {
				return server.requests.Load() >= 2
			})
			e.stop(t)
			if got := reports.statuses(); len(got) == 0 {
				t.Error("no check was reported")
			}
			for _, status := range reports.statuses() {
				if status != common_go.UpdateNotNeeded {
					t.Errorf("a check was reported as %s, want %s", status,
						common_go.UpdateNotNeeded)
				}
			}
			if logs.has(refreshLogged) {
				t.Error("the engine loaded the file")
			}
			if kept, err := os.ReadFile(data); err != nil || !bytes.Equal(kept, good) {
				t.Errorf("the data file was replaced (error %v)", err)
			}
		})
	}
}

// readPublished gives the published date of a data file, read from its
// header.
func readPublished(t *testing.T, data []byte) time.Time {
	t.Helper()
	e := newReloadTestEngine(t,
		WithDataFile(writeDataFile(t, data)),
		WithAutoUpdate(false),
		WithFileWatch(false))
	defer e.stop(t)
	return e.published()
}
