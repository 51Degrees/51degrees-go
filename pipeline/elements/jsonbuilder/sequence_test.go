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

package jsonbuilder

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// With no Sequence Element before it, the JSON Builder has no sequence
// number, so it fails, sets no JSON and says a Sequence Element needs to come
// first, whether the Pipeline returns or only records the error. A
// query.sequence value from the query string is not a sequence number.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/json-builder.md#process
func TestJSONBuilderNeedsSequenceNumber(t *testing.T) {
	for _, suppress := range []bool{false, true} {
		t.Run(fmt.Sprintf("suppress=%v", suppress), func(t *testing.T) {
			p, err := core.NewPipelineBuilder().
				SetSuppressProcessExceptions(suppress).
				AddFlowElement(newDeviceEngine()).
				AddFlowElement(New(nil, nil)).
				Build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			defer p.Close()
			fd := p.CreateFlowData()
			fd.AddEvidence("query.sequence", "3")
			returned := fd.Process()

			var failure error
			if suppress {
				if returned != nil {
					t.Fatalf("process returned %v, want the error recorded only", returned)
				}
				for _, fe := range fd.Errors() {
					failure = fe.Err
				}
			} else {
				failure = returned
			}
			if failure == nil {
				t.Fatal("the JSON Builder ran with no sequence number")
			}
			var config *core.ConfigurationError
			if !errors.As(failure, &config) {
				t.Errorf("the error %T is not a *core.ConfigurationError", failure)
			}
			if !strings.Contains(failure.Error(), "Sequence Element") {
				t.Errorf("the error %q does not say a Sequence Element is needed",
					failure)
			}
			if data, err := fd.Get(DataKey); err == nil {
				t.Errorf("the JSON Builder set its JSON %q", data.(*Data).JSON())
			}
		})
	}
}
