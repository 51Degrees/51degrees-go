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
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
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
// Run with -race, a reload that is not made on its own is reported, which
// ci/run-unit-tests.ps1 does on Linux.
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
