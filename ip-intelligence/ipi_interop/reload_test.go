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
	"os"
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
