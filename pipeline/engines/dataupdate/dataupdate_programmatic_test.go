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
)

// A programmatic check with no update URL loads the data file only when it
// is newer than the file last loaded, and otherwise reports the check as not
// needed.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/data-updates.md#programmatic-update
func TestAProgrammaticCheckWithNoNewerFileIsNotNeeded(t *testing.T) {
	eng := &fakeEngine{}
	svc := New(nil, nil)
	defer svc.Close()
	got := newReports()
	if err := svc.Register(eng, "Eng", Config{Identifier: "a",
		DataFilePath: writeFile(t, "DATA"), OnComplete: got.record}); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if err := svc.CheckForUpdate("a"); err != nil {
			t.Fatalf("check %d gave %v", i+1, err)
		}
	}
	list := got.list()
	if len(list) != 2 || list[0].Status != StatusSuccess ||
		list[1].Status != StatusNotNeeded {
		t.Errorf("the two checks were reported as %v, want %q then %q", list,
			StatusSuccess, StatusNotNeeded)
	}
	if got := eng.fileCount(); got != 1 {
		t.Errorf("the Engine was refreshed %d times, want once", got)
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
