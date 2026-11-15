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
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// timeoutStatus is the status of a watched file that could not be opened for
// reading in time, named as text so the tests build without the constant.
const timeoutStatus = Status("AUTO_UPDATE_TIMEOUT")

// releaseAfterSeconds is when the second test releases the watched file. It
// is longer than the 10 refresh tries 200 ms apart take, so only a wait for
// the file to become readable loads it.
const releaseAfterSeconds = 4

// heldFile stands for a data file another process holds, so it cannot be
// opened for reading until it is released.
type heldFile struct {
	path     string
	released atomic.Bool
}

// open fails for the held file until it is released.
func (h *heldFile) open(path string) error {
	if filepath.Clean(path) == filepath.Clean(h.path) && !h.released.Load() {
		return errors.New("the file is held by another process")
	}
	return nil
}

// holdWatchedFile makes the file at path unopenable until it is released, and
// sets how long the watcher waits for it, both put back when the test ends.
func holdWatchedFile(t *testing.T, path string, wait time.Duration) *heldFile {
	t.Helper()
	held := &heldFile{path: path}
	savedOpen, savedWait := openForReading, watchedFileReadableWait
	openForReading, watchedFileReadableWait = held.open, wait
	t.Cleanup(func() {
		openForReading, watchedFileReadableWait = savedOpen, savedWait
	})
	return held
}

// heldEngine cannot load the data file while the file is held, as an Engine
// reading a file another process holds cannot, and counts its loads.
type heldEngine struct {
	held  *heldFile
	loads atomic.Int32
}

func (e *heldEngine) RefreshFromFile(string) error {
	if !e.held.released.Load() {
		return errors.New("the data file could not be read")
	}
	e.loads.Add(1)
	return nil
}

func (e *heldEngine) RefreshFromBytes([]byte) error {
	e.loads.Add(1)
	return nil
}

// A watched file that cannot be opened for reading within the wait is
// reported as AUTO_UPDATE_TIMEOUT, is not loaded, and is not logged.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/data-updates.md?plain=1#L512
func TestAWatchedFileThatStaysHeldIsATimeout(t *testing.T) {
	path := writeFile(t, "INITIAL")
	held := holdWatchedFile(t, path, 500*time.Millisecond)
	engine := &heldEngine{held: held}
	svc, out := loggedService(nil)
	defer svc.Close()
	got := newReports()
	if err := svc.Register(engine, "Eng", Config{
		Identifier: "a", DataFilePath: path, OnComplete: got.record,
		FileSystemWatcherEnabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	replace(t, path, "NEWER", time.Now().Add(time.Hour))
	list := got.waitFor(t, timeoutStatus)
	if len(list) != 1 {
		t.Errorf("the reports were %v, want one of %q", list, timeoutStatus)
	}
	if n := engine.loads.Load(); n != 0 {
		t.Errorf("the Engine loaded the held file %d times, want none", n)
	}
	for _, level := range []string{"level=ERROR", "level=WARN"} {
		if strings.Contains(out.String(), level) {
			t.Errorf("the timeout was logged, want nothing:\n%s",
				out.String())
		}
	}
}

// A watched file released during the wait is loaded once it can be opened,
// and the check is reported as AUTO_UPDATE_SUCCESS.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/data-updates.md?plain=1#L512
func TestAWatchedFileReleasedDuringTheWaitIsLoaded(t *testing.T) {
	path := writeFile(t, "INITIAL")
	held := holdWatchedFile(t, path, time.Minute)
	engine := &heldEngine{held: held}
	svc, _ := loggedService(nil)
	defer svc.Close()
	got := newReports()
	if err := svc.Register(engine, "Eng", Config{
		Identifier: "a", DataFilePath: path, OnComplete: got.record,
		FileSystemWatcherEnabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	replace(t, path, "NEWER", time.Now().Add(time.Hour))
	release := time.AfterFunc(releaseAfterSeconds*time.Second, func() {
		held.released.Store(true)
	})
	defer release.Stop()
	list := got.waitFor(t, StatusSuccess)
	if len(list) != 1 {
		t.Errorf("the reports were %v, want one of %q", list, StatusSuccess)
	}
	if n := engine.loads.Load(); n != 1 {
		t.Errorf("the Engine loaded the file %d times, want once", n)
	}
}
