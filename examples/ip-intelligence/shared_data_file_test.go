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

package main

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

// With a licence key the update example renames the data file it downloads
// over its data file. The tests of other packages read the file
// _51DEGREES_IPI_PATH names at the same time, so onOwnDataFile, which
// TestMain runs this package's tests through, must leave that file as it was.
// The stand-in in testdata/update replaces its data file at once, as an
// update does, so the test needs no download and no wait.
func TestAnUpdateLeavesTheDataFileTheSettingsName(t *testing.T) {
	folder := t.TempDir()
	shared := filepath.Join(folder, "51Degrees-EnterpriseIpiV41.ipi")
	if err := os.WriteFile(shared, []byte("the shared data file"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("_51DEGREES_IPI_PATH", shared)
	t.Setenv("_51DEGREES_EVIDENCE_YAML", filepath.Join(folder, "evidence.yml"))
	before := sha256.Sum256([]byte("the shared data file"))
	code := onOwnDataFile(func() int {
		run(t, "testdata/update", nil)
		return 0
	})
	content, err := os.ReadFile(shared)
	if err != nil {
		t.Fatalf("an update removed %s: %v", shared, err)
	}
	if sha256.Sum256(content) != before {
		t.Errorf("an update replaced %s, the data file the settings name "+
			"and the other examples read, so it now holds %q", shared,
			content)
	}
	if code != 0 {
		t.Errorf("onOwnDataFile gave %d for tests that passed and left %s "+
			"as it was, want 0", code, shared)
	}
}
