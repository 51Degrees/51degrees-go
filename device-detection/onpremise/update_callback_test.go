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

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	common_go "github.com/51Degrees/51degrees-go/common/v4"
)

// reportLog keeps each report the engine makes and, for a success, the year
// of the data the engine answered from at that moment.
type reportLog struct {
	mu           sync.Mutex
	engine       *reloadTestEngine
	reports      []common_go.UpdateComplete
	successYears []int
}

func (l *reportLog) add(c common_go.UpdateComplete) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.reports = append(l.reports, c)
	if c.Status == common_go.UpdateSuccess && l.engine != nil {
		l.successYears = append(l.successYears, l.engine.published().Year())
	}
}

func (l *reportLog) watch(e *reloadTestEngine) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.engine = e
}

func (l *reportLog) count(status common_go.UpdateStatus) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, c := range l.reports {
		if c.Status == status {
			n++
		}
	}
	return n
}

// check fails unless every report named the data file, and each success came
// once the engine answered from the marked file.
func (l *reportLog) check(t *testing.T, dataFile string) {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, c := range l.reports {
		if c.DataFile != dataFile {
			t.Errorf("a report named '%s', want '%s'", c.DataFile, dataFile)
		}
	}
	for _, year := range l.successYears {
		if year != reloadTestMarkedYear {
			t.Errorf("success was reported while the engine answered from data "+
				"published in %d, before it loaded the new file", year)
		}
	}
}

// withPastNextUpdate returns the data file with the date it expects its next
// update moved to the marked year, because the first pull waits for that
// date, which a recently published file has not reached.
func withPastNextUpdate(t *testing.T, good []byte) []byte {
	t.Helper()
	e := newReloadTestEngine(t, WithDataFile(writeDataFile(t, good)), WithAutoUpdate(false), WithFileWatch(false))
	next := e.nextUpdateDate()
	e.stop(t)
	if !next.After(time.Now()) {
		return good
	}
	date := make([]byte, 4)
	binary.LittleEndian.PutUint16(date, uint16(next.Year()))
	date[2] = byte(next.Month())
	date[3] = byte(next.Day())
	i := bytes.Index(good[:reloadTestHeaderBytes], date)
	if i < 0 {
		t.Fatalf("next update date %v not found in the header", next)
	}
	moved := append([]byte{}, good...)
	binary.LittleEndian.PutUint16(moved[i:], reloadTestMarkedYear)
	return moved
}

// Each pull reports once it ends. A pull that finds new data ends when the
// engine has loaded it or failed to.
// See https://github.com/51Degrees/specifications/blob/6570727d2af6967390d3c4fbec77b9cf0540d3f3/pipeline-specification/features/data-updates.md#reporting-each-check
func TestUpdateCallbackHearsEachPull(t *testing.T) {
	good := withPastNextUpdate(t, readGoodFile(t))
	bad := unusableFiles(good)[1]
	server := newUpdateServer()
	defer server.Close()
	reports := &reportLog{}
	data := writeDataFile(t, good)
	e := newReloadTestEngine(t,
		WithDataFile(data),
		WithDataUpdateUrl(server.URL),
		WithPollingInterval(1),
		WithRandomization(0),
		WithFileWatch(false),
		WithCustomLogger(&reloadTestLog{}),
		WithUpdateCallback(reports.add))
	reports.watch(e)
	marked := markedGoodFile(t, good, e.published())

	waitFor(t, "a pull to report no update needed", func() bool {
		return reports.count(common_go.UpdateNotNeeded) > 0
	})
	server.publish(bad.data)
	waitFor(t, "the failed load to be reported", func() bool {
		return reports.count(common_go.UpdateRefreshFailed) > 0
	})
	server.publish(marked)
	waitFor(t, "the new data to be reported", func() bool {
		return reports.count(common_go.UpdateSuccess) > 0
	})
	reports.check(t, data)
}

// Each change the file watcher sees reports once the engine has loaded the
// file or failed to.
func TestUpdateCallbackHearsEachWatchedChange(t *testing.T) {
	good := readGoodFile(t)
	bad := unusableFiles(good)[1]
	reports := &reportLog{}
	data := writeDataFile(t, good)
	e := newReloadTestEngine(t,
		WithDataFile(data),
		WithAutoUpdate(false),
		WithFileWatch(true),
		WithCustomLogger(&reloadTestLog{}),
		WithUpdateCallback(reports.add))
	reports.watch(e)
	marked := markedGoodFile(t, good, e.published())

	replaceFile(t, data, bad.data)
	waitFor(t, "the failed load to be reported", func() bool {
		return reports.count(common_go.UpdateRefreshFailed) > 0
	})
	replaceFile(t, data, marked)
	waitFor(t, "the new data to be reported", func() bool {
		return reports.count(common_go.UpdateSuccess) > 0
	})
	reports.check(t, data)
}

// dataFileFor gives the path of a data file holding good, or of one that is
// not there yet when local is false.
func dataFileFor(t *testing.T, good []byte, local bool) string {
	t.Helper()
	data := writeDataFile(t, good)
	if !local {
		if err := os.Remove(data); err != nil {
			t.Fatal(err)
		}
	}
	return data
}

// The update on start-up reports before New returns, and New returns an error
// a callback raises.
func TestUpdateCallbackHearsTheUpdateOnStart(t *testing.T) {
	good := readGoodFile(t)
	for _, test := range []struct {
		name   string
		status int
		body   []byte
		local  bool
		want   common_go.UpdateStatus
	}{
		{"new data", http.StatusOK, good, false, common_go.UpdateSuccess},
		{"not modified", http.StatusNotModified, nil, true, common_go.UpdateNotNeeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			server, _ := recordingServer(test.status, test.body)
			defer server.Close()
			data := dataFileFor(t, good, test.local)
			reports := &reportLog{}
			newReloadTestEngine(t,
				WithDataFile(data),
				WithDataUpdateUrl(server.URL),
				WithUpdateOnStart(true),
				WithAutoUpdate(false),
				WithFileWatch(false),
				WithUpdateCallback(reports.add))
			reports.mu.Lock()
			got := append([]common_go.UpdateComplete{}, reports.reports...)
			reports.mu.Unlock()
			want := common_go.UpdateComplete{DataFile: data, Status: test.want}
			if len(got) != 1 || got[0] != want {
				t.Errorf("reported %v when New returned, want only %v", got, want)
			}
		})
		t.Run(test.name+", callback panics", func(t *testing.T) {
			server, _ := recordingServer(test.status, test.body)
			defer server.Close()
			data := dataFileFor(t, good, test.local)
			e, err := New(
				WithDataFile(data),
				WithDataUpdateUrl(server.URL),
				WithUpdateOnStart(true),
				WithAutoUpdate(false),
				WithFileWatch(false),
				WithUpdateCallback(func(common_go.UpdateComplete) { panic("callback test panic") }))
			if e != nil {
				e.Stop()
			}
			var p *common_go.UpdateCallbackPanic
			if !errors.As(err, &p) {
				t.Errorf("New gave %v, want the callback's panic", err)
			}
		})
	}
}

// A callback that panics on a watched change is logged with the error table's
// last row, and the engine still answers from the new file.
func TestUpdateCallbackPanicIsLogged(t *testing.T) {
	good := readGoodFile(t)
	logs := &reloadTestLog{}
	data := writeDataFile(t, good)
	e := newReloadTestEngine(t,
		WithDataFile(data),
		WithAutoUpdate(false),
		WithFileWatch(true),
		WithCustomLogger(logs),
		WithUpdateCallback(func(common_go.UpdateComplete) { panic("callback test panic") }))
	replaceFile(t, data, markedGoodFile(t, good, e.published()))
	waitFor(t, "the callback's panic to be logged", func() bool {
		return logs.has("An unhandled error occurred while checking for automatic updates for engine '"+
			engineTypeName+"'", "panic in an update callback: callback test panic")
	})
	if got := e.published().Year(); got != reloadTestMarkedYear {
		t.Errorf("the engine answers from data published in %d, want the new file's %d",
			got, reloadTestMarkedYear)
	}
}
