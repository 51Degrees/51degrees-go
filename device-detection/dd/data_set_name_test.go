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
	"path/filepath"
	"testing"
)

// The name of the loaded data file is its data tier, which is Lite for the
// free data file.
func TestGetDataSetName(t *testing.T) {
	testDifferentPerformanceProfiles("", func(manager *ResourceManager, t *testing.T) {
		name := GetDataSetName(manager)
		if name == "" {
			t.Error("the data file gave no name")
		}
		if filepath.Base(testDataFilePath) == testLiteDataFile && name != "Lite" {
			t.Errorf("the free data file gave the name %q, want Lite", name)
		}
	}, t)
}
