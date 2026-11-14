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
	"testing"
	"time"
)

// A programmatic check with no update URL loads the data file only when it
// is newer than the file the Engine holds, which at first is the file it was
// registered with, and otherwise reports the check as not needed.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/data-updates.md?plain=1#L336-L350
func TestAProgrammaticCheckWithNoNewerFileIsNotNeeded(t *testing.T) {
	path := writeFile(t, "DATA")
	eng := &fakeEngine{}
	svc := New(nil, nil)
	defer svc.Close()
	got := newReports()
	if err := svc.Register(eng, "Eng", Config{Identifier: "a",
		DataFilePath: path, OnComplete: got.record}); err != nil {
		t.Fatal(err)
	}

	check := func(name string) {
		t.Helper()
		if err := svc.CheckForUpdate("a"); err != nil {
			t.Fatalf("the check %s gave %v", name, err)
		}
	}
	check("of the file the Engine was registered with")
	replace(t, path, "NEWER", time.Now().Add(time.Hour))
	check("of a newer file")
	check("of the file just loaded")

	list := got.list()
	want := []Status{StatusNotNeeded, StatusSuccess, StatusNotNeeded}
	if len(list) != len(want) {
		t.Fatalf("the three checks were reported as %v, want %v", list, want)
	}
	for i, report := range list {
		if report.Status != want[i] {
			t.Errorf("check %d was reported as %q, want %q", i+1,
				report.Status, want[i])
		}
	}
	if eng.fileCount() != 1 || eng.lastFile() != "NEWER" {
		t.Errorf("the Engine loaded %q after %d refreshes, want the newer "+
			"file loaded once", eng.lastFile(), eng.fileCount())
	}
}

// A programmatic check for a data file the service has no configuration for
// gives the caller the no configuration status and reports nothing.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/data-updates.md#programmatic-update
func TestAProgrammaticCheckOfAnUnknownDataFileGivesNoConfiguration(t *testing.T) {
	svc := New(nil, nil)
	defer svc.Close()
	got := newReports()
	if err := svc.Register(&fakeEngine{}, "Eng", Config{Identifier: "a",
		DataFilePath: writeFile(t, "DATA"), OnComplete: got.record}); err != nil {
		t.Fatal(err)
	}

	err := svc.CheckForUpdate("b")
	var status interface{ Status() Status }
	if !errors.As(err, &status) ||
		status.Status() != "AUTO_UPDATE_NO_CONFIGURATION" {
		t.Errorf("CheckForUpdate gave %v, want an error whose status is "+
			"AUTO_UPDATE_NO_CONFIGURATION", err)
	}
	if list := got.list(); len(list) != 0 {
		t.Errorf("the check was reported as %v, want no report", list)
	}
}
