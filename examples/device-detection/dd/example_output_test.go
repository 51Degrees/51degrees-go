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
	"strings"
	"testing"
)

// A manager made from a data file held in memory detects, and detects again
// once it is reloaded from data held in memory.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/data-updates.md?plain=1#L219-L220
func TestReloadFromMemoryDetectsBeforeAndAfterTheReload(t *testing.T) {
	output := test(t, "reload_from_memory")
	for _, want := range []string{"Made from memory, the iPhone gives",
		"Reloaded from memory, the iPhone gives"} {
		if strings.Count(output, want) != 1 {
			t.Errorf("reload_from_memory does not print %q once", want)
		}
	}
	if strings.Count(output, "\tIsMobile: True") != 2 {
		t.Error("reload_from_memory does not give IsMobile True before " +
			"and after the reload")
	}
}

// The example reads values as their Go types.
func TestStronglyTypedGivesGoTypes(t *testing.T) {
	output := test(t, "strongly_typed")
	for _, typed := range []string{"IsMobile: true (bool)", " (int)"} {
		if !strings.Contains(output, typed) {
			t.Errorf("strongly_typed does not show %q", typed)
		}
	}
}
