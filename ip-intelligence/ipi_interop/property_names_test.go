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
	"strings"
	"testing"
)

// TestPropertyNamesDeprecatedMethods checks that the deprecated Release and
// For still serve a caller outside this repository. For gives the one table
// for results from the data set loaded at build and from a reload, and the
// names can still be read after Release.
func TestPropertyNamesDeprecatedMethods(t *testing.T) {
	good := readReloadTestFile(t)
	dir := t.TempDir()
	first := filepath.Join(dir, "data0.ipi")
	writeReloadTestFile(t, first, good)
	manager := NewResourceManager()
	defer manager.Free()
	if err := InitManagerFromFile(manager, *NewConfigIpi(Balanced), "", first); err != nil {
		t.Fatal(err)
	}
	names := NewPropertyNames(manager)
	var want []string
	for i := 0; names.Name(i) != ""; i++ {
		want = append(want, names.Name(i))
	}
	if len(want) == 0 {
		t.Fatal("the table names no property")
	}

	before := NewResultsIpi(manager)
	defer before.Free()
	next := filepath.Join(dir, "data1.ipi")
	writeReloadTestFile(t, next, good)
	if err := manager.ReloadFromFile(*NewConfigIpi(Balanced), "", next); err != nil {
		t.Fatal(err)
	}
	after := NewResultsIpi(manager)
	defer after.Free()

	for when, results := range map[string]*ResultsIpi{
		"before the reload": before,
		"after the reload":  after,
	} {
		if got := names.For(results); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("For results made %s gave %v, want %v", when, got, want)
		}
	}

	names.Release()
	names.Release()
	for i, name := range want {
		if got := names.Name(i); got != name {
			t.Errorf("Name(%d) after Release is %q, want %q", i, got, name)
		}
	}
}
