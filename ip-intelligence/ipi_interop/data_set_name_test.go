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

package ipi_interop

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// finalizerWaitSeconds is how long a test waits for the garbage collector to
// run the finalizers of what it collected.
const finalizerWaitSeconds = 10

// runFinalizers collects garbage and waits until the finalizers queued by
// two collections have run, so an Exception that was not freed panics while
// the test runs.
func runFinalizers(t *testing.T) {
	t.Helper()
	for i := 0; i < 2; i++ {
		done := make(chan struct{})
		// The sentinel is larger than the objects the tiny allocator
		// groups together, whose finalizers can wait for the whole group.
		sentinel := new([64]byte)
		runtime.SetFinalizer(sentinel, func(*[64]byte) { close(done) })
		runtime.GC()
		select {
		case <-done:
		case <-time.After(finalizerWaitSeconds * time.Second):
			t.Fatal("the garbage collector ran no finalizer")
		}
	}
}

// The name of the loaded data file is its data tier, which for the Asn data
// file already starts with the product prefix, and reading it frees what it
// used, so the garbage collector finds no Exception left unfreed.
func TestGetDataSetName(t *testing.T) {
	good := readReloadTestFile(t)
	path := filepath.Join(t.TempDir(), "data.ipi")
	writeReloadTestFile(t, path, good)
	manager := NewResourceManager()
	defer manager.Free()
	if err := InitManagerFromFile(manager, *NewConfigIpi(InMemory), "", path); err != nil {
		t.Fatal(err)
	}

	name := GetDataSetName(manager)
	runFinalizers(t)

	if name != "IPIV4Asn" {
		t.Errorf("%s gave the name %q, want IPIV4Asn", reloadTestDataFile, name)
	}
}
