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

package core

import (
	"os"
	"strings"
	"testing"
)

// The README says what the code comments say about sharing a Flow Data and
// its Element Data: who writes Element Data, that Close returns it to a pool,
// that a reference kept after Close is not supported, that the web middleware
// closes the Flow Data when the request ends, that a late write panics, and
// that Element Data reading native results takes no lock.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md#element-data
// and https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md#reusing-element-data
func TestTheReadmeSaysHowElementDataIsWrittenAndReused(t *testing.T) {
	readme, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(readme)), " ")
	for _, want := range []string{
		"written only by the Flow Element that provides it",
		"`FlowData.Close`",
		"returns its Element Data to a pool",
		"Keeping a reference to Element Data after its Flow Data is closed is not supported",
		"closes the Flow Data when the request ends",
		"panics with `core.ErrFlowDataFinished`",
		"panics with `core.ErrElementDataFinished`",
		"two Derived Property Elements MUST NOT be members of the same parallel group",
		"Element Data that reads its values from native results",
		"takes no lock and is not safe to use from several goroutines at once",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the README does not say %q", want)
		}
	}
}