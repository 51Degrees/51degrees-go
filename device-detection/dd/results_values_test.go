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

// mobileResults are the results of detecting the mobile User-Agent with a
// manager made with every Property, freed when the test ends.
func mobileResults(t *testing.T) *ResultsHash {
	t.Helper()
	manager := initResourceManager("", t)
	t.Cleanup(func() { freeResourceManager(manager, t) })
	results := NewResultsHash(manager, 1, 0)
	t.Cleanup(results.Free)
	if err := results.MatchUserAgent(uaMobile); err != nil {
		t.Fatalf("detection failed: %v", err)
	}
	return results
}

// Values gives each value of a Property for the detection, the same values
// ValuesStringByIndex joins, and no values and no error for an index with no
// Property.
func TestValuesGivesEachValueOfTheProperty(t *testing.T) {
	results := mobileResults(t)
	names := results.AvailableProperties()
	compared := 0
	for i, name := range names {
		joined, err := results.ValuesStringByIndex(i, "|")
		if err != nil {
			t.Fatalf("ValuesStringByIndex(%d) for %s failed: %v", i, name,
				err)
		}
		values, err := results.Values(i)
		if err != nil {
			t.Fatalf("Values(%d) for %s failed: %v", i, name, err)
		}
		if got := strings.Join(values, "|"); got != joined {
			t.Errorf("Values gave %q for %s, want %q as ValuesStringByIndex "+
				"gives", got, name, joined)
		}
		if joined != "" {
			compared++
		}
	}
	if compared == 0 {
		t.Fatalf("none of the %d Properties had a value to compare",
			len(names))
	}
	for _, index := range []int{-1, len(names)} {
		if values, err := results.Values(index); err != nil ||
			len(values) != 0 {
			t.Errorf("Values(%d) gave %v and %v, want no values and no "+
				"error", index, values, err)
		}
	}
}

// Route tracing is built into the C library only in a debug build, which
// this package does not compile, so TraceByIndex says so rather than giving
// an empty trace as if one had been recorded.
func TestTraceByIndexSaysTracingIsNotBuilt(t *testing.T) {
	if _, err := mobileResults(t).TraceByIndex(0); err == nil {
		t.Error("TraceByIndex gave no error, want one saying route tracing " +
			"is not built")
	}
}
