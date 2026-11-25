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
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"
)

// The published date read from the header alone is the date of the data set
// loaded from the same file.
func TestGetPublishedDateFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.ipi")
	writeReloadTestFile(t, path, readReloadTestFile(t))
	manager := NewResourceManager()
	defer manager.Free()
	config := NewConfigIpi(InMemory)
	if err := InitManagerFromFile(manager, *config, "", path); err != nil {
		t.Fatal(err)
	}
	want := GetPublishedDate(manager)

	got, err := GetPublishedDateFromFile(path)

	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(want) {
		t.Errorf("the header gives %v, and the loaded data set %v", got, want)
	}
}

// A file that is missing, not a data file or of another version gives an
// error and the zero time.
func TestGetPublishedDateFromFileRefusesAFileItCannotRead(t *testing.T) {
	good := readReloadTestFile(t)
	version := append([]byte{}, good...)
	// The header starts with the major version of the format.
	binary.LittleEndian.PutUint32(version, 3)
	dir := t.TempDir()
	for _, c := range []struct {
		name   string
		data   []byte
		reason string
	}{
		{"missing", nil, "could not be found"},
		{"not a data file", []byte("Bad data"), "not in the correct format"},
		{"wrong version", version, "unsupported version"},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "-")+".ipi")
			if c.data != nil {
				writeReloadTestFile(t, path, c.data)
			}

			got, err := GetPublishedDateFromFile(path)

			if err == nil {
				t.Fatalf("no error, and the date %v", got)
			}
			if !got.IsZero() {
				t.Errorf("the date is %v, want the zero time", got)
			}
			if !strings.Contains(err.Error(), c.reason) {
				t.Errorf("error %q does not say %q", err, c.reason)
			}
		})
	}
}
