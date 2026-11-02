/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2019 51 Degrees Mobile Experts Limited, 5 Charlotte Close,
 * Caversham, Reading, Berkshire, United Kingdom RG4 7BY.
 *
 * This Original Work is licensed under the European Union Public Licence (EUPL)
 * v.1.2 and is subject to its terms as set out below.
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
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const (
	// reloadTestDataFile is the small data file from the ip-intelligence-data
	// repository, which CI places in the repository root.
	reloadTestDataFile = "51Degrees-IPIV4AsnIpiV41.ipi"
	reloadTestReloads  = 5
	// reloadTestFileMargin allows for files the test opens for itself, and is
	// fewer than the files one data set keeps open.
	reloadTestFileMargin = 5
)

func readReloadTestFile(t *testing.T) []byte {
	t.Helper()
	good, err := os.ReadFile(filepath.Join("..", reloadTestDataFile))
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal(err)
		}
		t.Skipf("copy %s from the ip-intelligence-data repository into "+
			"the repository root to run this test: %v", reloadTestDataFile, err)
	}
	return good
}

func writeReloadTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

// openFiles counts the files the process has open, or returns -1 on Windows,
// which has no list of them. Only the names are read, as a file can close
// between being listed and being examined.
func openFiles(t *testing.T) int {
	t.Helper()
	if runtime.GOOS == "windows" {
		return -1
	}
	dir := "/dev/fd"
	if runtime.GOOS == "linux" {
		dir = "/proc/self/fd"
	}
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		t.Fatal(err)
	}
	return len(names)
}

// A file of the wrong version is refused by the status alone, so a reload of
// the original file must still return an error.
func TestReloadFromOriginalFileReportsStatusFailure(t *testing.T) {
	good := readReloadTestFile(t)
	bad := append([]byte{}, good...)
	// The header starts with the major version of the format.
	binary.LittleEndian.PutUint32(bad, 3)

	master := filepath.Join(t.TempDir(), "master.ipi")
	writeReloadTestFile(t, master, good)
	manager := NewResourceManager()
	defer manager.Free()
	// In memory, so that the master file can be replaced on Windows.
	config := NewConfigIpi(InMemory)
	if err := InitManagerFromFile(manager, *config, "", master); err != nil {
		t.Fatal(err)
	}

	writeReloadTestFile(t, master+".new", bad)
	if err := os.Rename(master+".new", master); err != nil {
		t.Fatal(err)
	}
	err := manager.ReloadFromOriginalFile()
	if err == nil {
		t.Fatal("the reload returned no error")
	}
	if !strings.Contains(err.Error(), "unsupported version") ||
		!strings.Contains(err.Error(), master) {
		t.Errorf("error %q does not name the version and %s", err, master)
	}
}

// Each reload from a file must free the data set it replaces, closing that
// data set's files, so that the copy it was loaded from can be deleted.
func TestReloadFromFileFreesReplacedDataSet(t *testing.T) {
	good := readReloadTestFile(t)
	dir := t.TempDir()
	previous := filepath.Join(dir, "data0.ipi")
	writeReloadTestFile(t, previous, good)
	manager := NewResourceManager()
	defer manager.Free()
	config := NewConfigIpi(Balanced)
	if err := InitManagerFromFile(manager, *config, "", previous); err != nil {
		t.Fatal(err)
	}

	before := openFiles(t)
	for i := 1; i <= reloadTestReloads; i++ {
		next := filepath.Join(dir, "data"+strconv.Itoa(i)+".ipi")
		writeReloadTestFile(t, next, good)
		if err := manager.ReloadFromFile(*config, "", next); err != nil {
			t.Fatal(err)
		}
		// Windows cannot delete a file that a data set still has open.
		if err := os.Remove(previous); err != nil {
			t.Errorf("reload %d left %s in use: %v", i, previous, err)
		}
		previous = next
	}
	if before >= 0 {
		after := openFiles(t)
		t.Logf("%d files open after %d reloads, %d before",
			after, reloadTestReloads, before)
		if after > before+reloadTestFileMargin {
			t.Errorf("%d files open after %d reloads, %d before",
				after, reloadTestReloads, before)
		}
	}
}
