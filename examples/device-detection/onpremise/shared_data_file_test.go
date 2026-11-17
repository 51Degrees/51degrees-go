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

package onpremise

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

// The update example is tested with the settings CI gives every example, and
// with a licence key it renames the Enterprise data file it downloads over
// the data file _51DEGREES_DD_PATH names. The other examples, tested at the
// same time, read that file as the free one, so it must be left as it was.
// The stand-in in testdata/update replaces its data file at once, as an
// update does, so the test needs no download and no wait.
func TestAnUpdateLeavesTheDataFileTheSettingsName(t *testing.T) {
	shared := filepath.Join(t.TempDir(), "51Degrees-LiteV4.1.hash")
	if err := os.WriteFile(shared, []byte("the free data file"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("_51DEGREES_DD_PATH", shared)
	before := sha256.Sum256([]byte("the free data file"))
	test(t, "testdata/update")
	content, err := os.ReadFile(shared)
	if err != nil {
		t.Fatalf("an update removed %s: %v", shared, err)
	}
	if sha256.Sum256(content) != before {
		t.Errorf("an update replaced %s, the data file the settings name "+
			"and the other examples read, so it now holds %q", shared,
			content)
	}
}
