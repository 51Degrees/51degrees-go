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

//#include <string.h>
//#include "ip-intelligence-cxx.h"
import "C"

// NewEvidenceWithCapacity creates Evidence that values can be added to, with
// room for capacity of them. The caller calls Free on it once the results
// made from it are populated.
func NewEvidenceWithCapacity(capacity int) *Evidence {
	if capacity < 1 {
		capacity = 1
	}
	return &Evidence{
		cEvidence: make([]CEvidence, 0, capacity),
		CPtr:      C.fiftyoneDegreesEvidenceCreate(C.uint32_t(capacity)),
	}
}

// ResultsIpiFromEvidence populates the results from the Evidence. The native
// code chooses the IP address, trying the query keys and then the server
// keys, each in the data file's order, and using the first value it reads as
// one whole address. When it reads none the results hold no values, which is
// not an error.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L150-L175
func (r *ResultsIpi) ResultsIpiFromEvidence(evidence *Evidence) error {
	exception := NewException()
	defer exception.Free()

	C.fiftyoneDegreesResultsIpiFromEvidence(
		r.CPtr,
		evidence.CPtr,
		exception.CPtr,
	)

	if !exception.IsOkay() {
		return messageError(C.ExceptionGetMessage(exception.CPtr))
	}

	return nil
}
