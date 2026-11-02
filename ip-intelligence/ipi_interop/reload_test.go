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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dataFileVariable names the environment variable holding the path of an IP
// Intelligence data file. Tests that need a loaded data set skip without it.
const dataFileVariable = "51DEGREES_IPI_PATH"

// dataFilePath returns the data file named by dataFileVariable, or skips the
// test when the variable is not set.
func dataFilePath(t *testing.T) string {
	path := os.Getenv(dataFileVariable)
	if path == "" {
		t.Skipf("%s is not set", dataFileVariable)
	}
	return path
}

// TestReloadFromFileFreesDataSet checks that each reload frees the data set
// it replaces, so nothing the library allocated is left once the manager is
// freed.
func TestReloadFromFileFreesDataSet(t *testing.T) {
	path := dataFilePath(t)
	config := *NewConfigIpi(LowMemory)

	var err error
	left := allocatedDuring(func() {
		manager := NewResourceManager()
		defer manager.Free()
		if err = InitManagerFromFile(manager, config, "", path); err != nil {
			return
		}
		for i := 0; i < 3 && err == nil; i++ {
			err = manager.ReloadFromFile(config, "", path)
		}
	})
	if err != nil {
		t.Fatalf("loading %s: %v", path, err)
	}
	if left != 0 {
		t.Errorf("%d bytes still allocated after three reloads and Free", left)
	}
}

// load returns a manager loaded from path with the given profile.
func load(t *testing.T, path string, profile PerformanceProfile) *ResourceManager {
	manager := NewResourceManager()
	t.Cleanup(manager.Free)
	if err := InitManagerFromFile(
		manager, *NewConfigIpi(profile), "", path); err != nil {
		t.Fatalf("InitManagerFromFile() failed: %v", err)
	}
	return manager
}

// copyFile copies the file at from to a new file at to.
func copyFile(t *testing.T, from string, to string) {
	src, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := os.Create(to)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		t.Fatal(err)
	}
	if err := dst.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestReloadFromFileReportsStatus checks that a reload that fails with a
// status code returns an error naming the file. A file of zeros fails the
// version check in the data file's header, which is a status code.
func TestReloadFromFileReportsStatus(t *testing.T) {
	manager := load(t, dataFilePath(t), LowMemory)

	zeros := filepath.Join(t.TempDir(), "zeros.ipi")
	if err := os.WriteFile(zeros, make([]byte, 65536), 0o600); err != nil {
		t.Fatal(err)
	}
	err := manager.ReloadFromFile(*NewConfigIpi(LowMemory), "", zeros)
	if err == nil {
		t.Fatal("ReloadFromFile() reported success for a file of zeros")
	}
	if !strings.Contains(err.Error(), zeros) {
		t.Errorf("error does not name %q: %v", zeros, err)
	}
}

// TestReloadFromOriginalFileReportsStatus checks that reloading once the
// original data file has gone returns an error naming that file. The in
// memory profile reports the missing file as a status code. The test copies
// the data file and loads the copy in memory, so a Lite data file keeps it
// quick.
func TestReloadFromOriginalFileReportsStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "original.ipi")
	copyFile(t, dataFilePath(t), path)
	manager := load(t, path, InMemory)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	err := manager.ReloadFromOriginalFile()
	if err == nil {
		t.Fatal("ReloadFromOriginalFile() reported success without its file")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error does not name %q: %v", path, err)
	}
}
