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

// These tests follow the refresh on demand an Engine offers.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/data-updates.md?plain=1#L36-L41

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	// refreshTestProcessors and refreshTestLookups are how many goroutines
	// process while others refresh, and how many lookups each makes.
	refreshTestProcessors = 4
	refreshTestLookups    = 200
	// refreshTestRefreshers and refreshTestRefreshes are how many goroutines
	// refresh at once, and how many refreshes each makes.
	refreshTestRefreshers = 2
	refreshTestRefreshes  = 3
)

// RefreshData loads a replaced data file now. When the file cannot be used
// it returns the error and the Engine goes on with the data it had, and a
// later good file is still loaded.
func TestRefreshDataOnDemand(t *testing.T) {
	good := readGoodFile(t)
	for _, temp := range []bool{true, false} {
		t.Run(fmt.Sprintf("temporary copy %v", temp), func(t *testing.T) {
			if !temp && runtime.GOOS == "windows" {
				t.Skip("Windows cannot replace a file the data set has open")
			}
			path := writeDataFile(t, good)
			e := newReloadTestEngine(t, WithDataFile(path),
				WithAutoUpdate(false), WithFileWatch(false),
				WithTempDataCopy(temp))
			answer := e.asnName(t)
			published := e.published()

			for _, bad := range unusableFiles(good) {
				replaceFile(t, path, bad.data)
				err := e.RefreshData()
				if err == nil || !strings.Contains(err.Error(), bad.reason) {
					t.Errorf("%s: RefreshData gave %v, want an error saying %q",
						bad.name, err, bad.reason)
				}
				if got := e.asnName(t); got != answer {
					t.Errorf("%s: AsnName is %q after the failed refresh, was %q",
						bad.name, got, answer)
				}
				if got := e.published(); !got.Equal(published) {
					t.Errorf("%s: published date is %v after the failed refresh, was %v",
						bad.name, got, published)
				}
			}

			replaceFile(t, path, markedGoodFile(t, good, published))
			if err := e.RefreshData(); err != nil {
				t.Fatalf("RefreshData gave %v for a good file", err)
			}
			if got := e.published().Year(); got != reloadTestMarkedYear {
				t.Errorf("published year is %d after the refresh, want %d",
					got, reloadTestMarkedYear)
			}
		})
	}
}

// RefreshData is safe to call while other goroutines process and refresh.
func TestRefreshDataWhileProcessing(t *testing.T) {
	e := newReloadTestEngine(t, WithDataFile(writeDataFile(t, readGoodFile(t))),
		WithAutoUpdate(false), WithFileWatch(false))
	errs := make(chan error, refreshTestProcessors*refreshTestLookups+
		refreshTestRefreshers*refreshTestRefreshes)
	var wg sync.WaitGroup
	for i := 0; i < refreshTestProcessors; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < refreshTestLookups; j++ {
				if _, err := e.Process(reloadTestAddress); err != nil {
					errs <- err
				}
			}
		}()
	}
	for i := 0; i < refreshTestRefreshers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < refreshTestRefreshes; j++ {
				if err := e.RefreshData(); err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// otherCase gives s with the case of each ASCII letter swapped, so it keeps
// its length.
func otherCase(s string) string {
	b := []byte(s)
	for i, c := range b {
		switch {
		case 'a' <= c && c <= 'z':
			b[i] = c - 'a' + 'A'
		case 'A' <= c && c <= 'Z':
			b[i] = c - 'A' + 'a'
		}
	}
	return string(b)
}

// stringRecord is s as the data file holds a string, being its length with
// the terminating null as a little endian 16 bit number, then its bytes and
// the null.
func stringRecord(s string) []byte {
	record := binary.LittleEndian.AppendUint16(nil, uint16(len(s)+1))
	return append(append(record, s...), 0)
}

// valueMarkedFile returns the marked good file with each string of values
// held in the other case, so Data made from it gives a different answer from
// Data made from the good file.
func valueMarkedFile(
	t *testing.T, good []byte, published time.Time, values []string) []byte {
	t.Helper()
	marked := markedGoodFile(t, good, published)
	for _, value := range values {
		if otherCase(value) == value {
			t.Fatalf("%q has no letter to change", value)
		}
		from := stringRecord(value)
		if !bytes.Contains(good, from) {
			t.Fatalf("%q is not held as a string in the data file", value)
		}
		marked = bytes.ReplaceAll(marked, from,
			stringRecord(otherCase(value)))
	}
	return marked
}

// asnNameOf reads the AsnName values of d.
func asnNameOf(d *Data) ([]string, error) {
	values, err := d.Values("AsnName")
	if err != nil {
		return nil, err
	}
	names := make([]string, len(values))
	for i, v := range values {
		names[i] = fmt.Sprint(v.Value)
	}
	return names, nil
}

// asnValues makes Data for the test address and reads its AsnName values.
func (e *reloadTestEngine) asnValues(t *testing.T) []string {
	t.Helper()
	d, err := e.ProcessData(reloadTestAddress)
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	defer d.Free()
	names, err := asnNameOf(d)
	if err != nil {
		t.Fatalf("reading AsnName failed: %v", err)
	}
	if len(names) == 0 {
		t.Fatalf("%s has no AsnName", reloadTestAddress)
	}
	return names
}

// otherCaseAll gives each of values in the other case, joined by commas.
func otherCaseAll(values []string) string {
	changed := make([]string, len(values))
	for i, value := range values {
		changed[i] = otherCase(value)
	}
	return strings.Join(changed, ",")
}

// dataMade is the data a Data was made from, as far as the refreshes seen
// before and after it was made tell.
type dataMade int

const (
	madeFromEither dataMade = iota
	madeFromOld
	madeFromUpdated
)

// readCheck checks the values read from Data against the data it was made
// from. It counts the reads and the wrong ones and keeps the first wrong
// one, so a failure is reported once.
type readCheck struct {
	old, updated string
	reads, wrong atomic.Int32
	first        sync.Once
	what         string
}

func (c *readCheck) read(d *Data, made dataMade) {
	c.reads.Add(1)
	names, err := asnNameOf(d)
	got := strings.Join(names, ",")
	switch {
	case err != nil:
		c.fail("reading AsnName failed: %v", err)
	case made == madeFromOld && got != c.old:
		c.fail("AsnName is %q from Data made before any refresh, want %q",
			got, c.old)
	case made == madeFromUpdated && got != c.updated:
		c.fail("AsnName is %q from Data made after a refresh, want %q",
			got, c.updated)
	case got != c.old && got != c.updated:
		c.fail("AsnName is %q, want %q or %q", got, c.old, c.updated)
	}
}

func (c *readCheck) fail(format string, v ...interface{}) {
	c.wrong.Add(1)
	c.first.Do(func() { c.what = fmt.Sprintf(format, v...) })
}

func (c *readCheck) report(t *testing.T) {
	t.Helper()
	if n := c.wrong.Load(); n > 0 {
		t.Errorf("%d of %d reads were wrong, the first: %s",
			n, c.reads.Load(), c.what)
	}
}

// keptData is Data kept to be read once the refreshes have ended.
type keptData struct {
	data *Data
	made dataMade
}

// Data made before and while other goroutines refresh gives the values of the
// data it was made from, when first read while the refreshes run and after
// they end. Data keeps a value once read, so each Data is read once. The
// refreshes load a file whose AsnName values differ, so a value read through
// any other data is seen.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/data-updates.md?plain=1#L47-L52
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/automated-testing.md?plain=1#L183-L185
func TestRefreshDataWhileReadingValues(t *testing.T) {
	good := readGoodFile(t)
	path := writeDataFile(t, good)
	e := newReloadTestEngine(t, WithDataFile(path), WithAutoUpdate(false),
		WithFileWatch(false), WithTempDataCopy(true))
	old := e.asnValues(t)
	check := &readCheck{
		old:     strings.Join(old, ","),
		updated: otherCaseAll(old),
	}
	before := make([]*Data, refreshTestProcessors)
	defer func() {
		for _, d := range before {
			if d != nil {
				d.Free()
			}
		}
	}()
	for i := range before {
		d, err := e.ProcessData(reloadTestAddress)
		if err != nil {
			t.Fatalf("lookup failed: %v", err)
		}
		before[i] = d
	}
	replaceFile(t, path, valueMarkedFile(t, good, e.published(), old))

	// started counts the refreshes begun and finished those that loaded the
	// marked file.
	var started, finished atomic.Int32
	refreshed := make(chan struct{})
	var processors sync.WaitGroup
	for i := 0; i < refreshTestProcessors; i++ {
		processors.Add(1)
		go func(first *Data) {
			defer processors.Done()
			kept := make([]keptData, 0, refreshTestLookups)
			for j := 0; j < refreshTestLookups; j++ {
				if first != nil && finished.Load() > 0 {
					check.read(first, madeFromOld)
					first.Free()
					first = nil
				}
				made := madeFromEither
				if finished.Load() > 0 {
					made = madeFromUpdated
				}
				now, err := e.ProcessData(reloadTestAddress)
				if err != nil {
					check.fail("lookup failed: %v", err)
					continue
				}
				later, err := e.ProcessData(reloadTestAddress)
				if err != nil {
					now.Free()
					check.fail("lookup failed: %v", err)
					continue
				}
				if made == madeFromEither && started.Load() == 0 {
					made = madeFromOld
				}
				check.read(now, made)
				now.Free()
				kept = append(kept, keptData{later, made})
			}
			<-refreshed
			if first != nil {
				check.read(first, madeFromOld)
				first.Free()
			}
			for _, k := range kept {
				check.read(k.data, k.made)
				k.data.Free()
			}
		}(before[i])
	}
	var refreshers sync.WaitGroup
	for i := 0; i < refreshTestRefreshers; i++ {
		refreshers.Add(1)
		go func() {
			defer refreshers.Done()
			for j := 0; j < refreshTestRefreshes; j++ {
				started.Add(1)
				if err := e.RefreshData(); err != nil {
					t.Error(err)
					continue
				}
				finished.Add(1)
			}
		}()
	}
	refreshers.Wait()
	close(refreshed)
	processors.Wait()
	check.report(t)
	if got := strings.Join(e.asnValues(t), ","); got != check.updated {
		t.Errorf("AsnName is %q after the refreshes, want %q from the "+
			"marked file", got, check.updated)
	}
}

// Data made before a refresh gives the values of the data it was made from,
// however many refreshes happen before it is freed.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/data-updates.md?plain=1#L47-L52
func TestDataMadeBeforeARefreshKeepsItsData(t *testing.T) {
	good := readGoodFile(t)
	path := writeDataFile(t, good)
	e := newReloadTestEngine(t, WithDataFile(path), WithAutoUpdate(false),
		WithFileWatch(false), WithTempDataCopy(true))
	old := e.asnValues(t)
	d, err := e.ProcessData(reloadTestAddress)
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	defer d.Free()
	replaceFile(t, path, valueMarkedFile(t, good, e.published(), old))
	for i := 0; i < refreshTestRefreshes; i++ {
		if err := e.RefreshData(); err != nil {
			t.Fatalf("RefreshData gave %v for the marked file", err)
		}
	}
	if got := strings.Join(e.asnValues(t), ","); got != otherCaseAll(old) {
		t.Errorf("AsnName is %q after the refreshes, want %q from the "+
			"marked file", got, otherCaseAll(old))
	}
	names, err := asnNameOf(d)
	if err != nil {
		t.Fatalf("reading AsnName failed: %v", err)
	}
	if got := strings.Join(names, ","); got != strings.Join(old, ",") {
		t.Errorf("AsnName is %q from Data made before the refreshes, want "+
			"%q from the data it was made from", got, strings.Join(old, ","))
	}
	d.Free()
}

// loadCounter is a logger that counts the reloads reporting at once. A
// reload reports the file it loaded before it ends, and the counter holds
// each report for a moment, so two reloads running at once are seen
// reporting at once.
type loadCounter struct {
	inProgress atomic.Int32
	most       atomic.Int32
	loads      atomic.Int32
}

// loadReportMilliseconds is how long the counter holds each report.
const loadReportMilliseconds = 20

func (c *loadCounter) Printf(format string, v ...interface{}) {
	if !strings.HasPrefix(fmt.Sprintf(format, v...), "data file loaded from ") {
		return
	}
	now := c.inProgress.Add(1)
	for {
		most := c.most.Load()
		if now <= most || c.most.CompareAndSwap(most, now) {
			break
		}
	}
	c.loads.Add(1)
	time.Sleep(loadReportMilliseconds * time.Millisecond)
	c.inProgress.Add(-1)
}

// Reloads asked for from several goroutines at once are made one at a time.
func TestRefreshDataReloadsOneAtATime(t *testing.T) {
	counter := &loadCounter{}
	e := newReloadTestEngine(t, WithDataFile(writeDataFile(t, readGoodFile(t))),
		WithAutoUpdate(false), WithFileWatch(false), WithCustomLogger(counter))
	var wg sync.WaitGroup
	for i := 0; i < refreshTestRefreshers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < refreshTestRefreshes; j++ {
				if err := e.RefreshData(); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	// The first load, when the Engine was made, and each refresh.
	want := int32(1 + refreshTestRefreshers*refreshTestRefreshes)
	if got := counter.loads.Load(); got != want {
		t.Errorf("%d loads were reported, want %d", got, want)
	}
	if most := counter.most.Load(); most != 1 {
		t.Errorf("%d reloads ran at once, want one at a time", most)
	}
}

// RefreshData on an Engine that has stopped refuses, as there is no data set
// left to refresh.
func TestRefreshDataAfterStop(t *testing.T) {
	e := newReloadTestEngine(t, WithDataFile(writeDataFile(t, readGoodFile(t))),
		WithAutoUpdate(false), WithFileWatch(false))
	e.stop(t)
	if err := e.RefreshData(); err == nil {
		t.Error("RefreshData gave no error after Stop")
	}
}
