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

package ipi_onpremise

import (
	"errors"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// The AspectEngine's Element Data cannot be written once it has processed.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/thread-safety.md?plain=1#L156-L158
func TestAspectEngineDataIsReadOnlyOnceProcessed(t *testing.T) {
	a := newTestAspectEngine(t)
	query, _ := acceptedNames(t, a)
	data := processEvidence(t, newTestPipeline(t, a),
		map[string]any{query: aspectTestAddresses[0]})
	defer func() {
		r := recover()
		err, ok := r.(error)
		if !ok || !errors.Is(err, core.ErrElementDataFinished) {
			t.Errorf("a write after processing panicked with %v, want "+
				"core.ErrElementDataFinished", r)
		}
	}()
	data.Set("written-after-processing", true)
}
