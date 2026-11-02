/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2019 51 Degrees Mobile Experts Limited, 5 Charlotte Close,
 * Caversham, Reading, Berkshire, United Kingdom RG4 7BY.
 *
 * This Original Work is licensed under the European Union Public Licence (EUPL)
 * v.1.2 and is subject to its terms as set out below.
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
import "unsafe"

type EvidencePrefix C.fiftyoneDegreesEvidencePrefix

// Header Key required by engine
type EvidenceKey struct {
	Prefix EvidencePrefix
	Key    string
}

// C type Evidence
type CEvidence struct {
	key   *C.char
	value *C.char
}

// Evidence structure
type Evidence struct {
	cEvidence []CEvidence
	CPtr      *C.EvidenceKeyValuePairArray
}

func NewEvidence() *Evidence {
	return &Evidence{
		cEvidence: make([]CEvidence, 0, 0),
	}
}

// Free frees the evidence resources allocated in the C layer. This matches the
// C API fiftyoneDegreesEvidenceFree
func (evidence *Evidence) Free() {
	// Free the tracked C evidence strings
	if evidence.cEvidence != nil {
		// Free each cstring in the evidence
		for _, e := range evidence.cEvidence {
			C.free(unsafe.Pointer(e.key))
			C.free(unsafe.Pointer(e.value))
		}
		evidence.cEvidence = nil
	}

	// Free the C resources
	if evidence.CPtr != nil {
		C.EvidenceFree(evidence.CPtr)
		evidence.CPtr = nil
	}
}

// Count return number of evidence in Evidence object
func (evidence *Evidence) Count() int {
	return int(evidence.CPtr.count)
}

// Add adds a new evidence to the object. This matches the C API
// fiftyoneDegreesEvidenceAddString
func (evidence *Evidence) Add(
	prefix EvidencePrefix,
	key string,
	value string) error {
	cKey := C.CString(key)
	cValue := C.CString(value)
	// Add it to the tracked map
	evidence.cEvidence = append(evidence.cEvidence, CEvidence{cKey, cValue})
	C.EvidenceAddString(
		evidence.CPtr,
		C.fiftyoneDegreesEvidencePrefix(prefix),
		cKey,
		cValue,
	)

	return nil
}
