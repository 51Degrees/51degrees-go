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

import "testing"

// The C library sets an exception when a lookup fails and never clears it.
// A failed lookup for one property must not give false for the properties
// looked up after it, or a Latitude after the failure would not be rounded
// and a weighted property after it would lose its weight.
func TestPropertyCheckAfterFailedLookup(t *testing.T) {
	exception := NewException()
	defer exception.Free()
	// Like isCoordinate and isPropertyWeighted, the check gives false when
	// the exception is set once its lookup has run. Its lookup fails for
	// property 0 only.
	check := newPropertyCheck(exception,
		func(requiredIndex int, exception *Exception) bool {
			if requiredIndex == 0 {
				exception.CPtr.status = 1 // Any status other than not set
			}
			return exception.IsOkay()
		})
	if check.answer(0) {
		t.Error("property 0, whose lookup failed, gave true")
	}
	if !check.answer(1) {
		t.Error("property 1 gave false after the lookup for property 0 failed")
	}
}
