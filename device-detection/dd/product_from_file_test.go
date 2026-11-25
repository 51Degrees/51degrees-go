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

package dd

import (
	"os"
	"path/filepath"
	"testing"
)

// The product name read from the header of a data file, without loading it,
// is V4 followed by the name the loaded data file gives itself, and the
// manager's own memory tracking finds nothing allocated by the read.
func TestGetProductFromFile(t *testing.T) {
	var product string
	var err error
	left := allocatedDuring(func() {
		product, err = GetProductFromFile(testDataFilePath)
	})
	if err != nil {
		t.Fatalf("the product of %s could not be read: %v", testDataFilePath, err)
	}
	if left != 0 {
		t.Errorf("%d bytes of native memory were not freed", left)
	}
	testDifferentPerformanceProfiles("", func(manager *ResourceManager, t *testing.T) {
		if want := "V4" + GetDataSetName(manager); product != want {
			t.Errorf("the header names the product %q, want %q", product, want)
		}
	}, t)
	if filepath.Base(testDataFilePath) == testLiteDataFile && product != "V4Lite" {
		t.Errorf("the free data file is of the product %q, want V4Lite", product)
	}
}

// A file that is missing, or is not a data file, has no product, and the
// error says why.
func TestGetProductFromFileWithNoDataFile(t *testing.T) {
	folder := t.TempDir()
	notData := filepath.Join(folder, "not-data.hash")
	if err := os.WriteFile(notData, []byte("not a data file"), 0644); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"no file":      filepath.Join(folder, "missing.hash"),
		"no data file": notData,
		"no path":      "",
	} {
		t.Run(name, func(t *testing.T) {
			product, err := GetProductFromFile(path)
			if err == nil || err.Error() == "" {
				t.Errorf("no error was given, want one that says why")
			}
			if product != "" {
				t.Errorf("the product is %q, want none", product)
			}
		})
	}
}
