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
	"os"
	"path/filepath"
	"testing"
)

// The product name read from the header of a data file, without loading it,
// is the name the loaded data file gives itself, which for the Asn data file
// already starts with the product prefix.
func TestGetProductFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.ipi")
	writeReloadTestFile(t, path, readReloadTestFile(t))

	product, err := GetProductFromFile(path)
	if err != nil {
		t.Fatalf("the product of %s could not be read: %v", reloadTestDataFile, err)
	}

	manager := NewResourceManager()
	defer manager.Free()
	if err := InitManagerFromFile(manager, *NewConfigIpi(InMemory), "", path); err != nil {
		t.Fatal(err)
	}
	if want := GetDataSetName(manager); product != want {
		t.Errorf("the header names the product %q, want %q", product, want)
	}
	if product != "IPIV4Asn" {
		t.Errorf("%s is of the product %q, want IPIV4Asn", reloadTestDataFile, product)
	}
}

// A file that is missing, or is not a data file, has no product, and the
// error says why.
func TestGetProductFromFileWithNoDataFile(t *testing.T) {
	folder := t.TempDir()
	notData := filepath.Join(folder, "not-data.ipi")
	if err := os.WriteFile(notData, []byte("not a data file"), 0644); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"no file":      filepath.Join(folder, "missing.ipi"),
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
