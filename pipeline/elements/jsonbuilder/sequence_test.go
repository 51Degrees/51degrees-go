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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/sequence"
)

// The JSON Builder takes the sequence number from the integer in the
// query.sequence Evidence, which the Sequence Element writes. With an integer
// it builds the JSON. With none, or with a value that is not an integer such
// as the text a query string gives, it fails, sets no JSON and says a
// Sequence Element needs to come first, whether the Pipeline returns or only
// records the error.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/json-builder.md#process
func TestJSONBuilderSequenceNumber(t *testing.T) {
	cases := []struct {
		name string
		// withSequence puts a Sequence Element before the JSON Builder.
		withSequence bool
		// value is the query.sequence Evidence added before processing, or
		// nil for none.
		value any
		// wantJS is whether javascriptProperties is written, or nil when the
		// JSON Builder must fail.
		wantJS *bool
	}{
		{"integer from the Sequence Element", true, nil, ptr(true)},
		{"integer from the Sequence Element at the limit", true, "9", ptr(false)},
		{"integer added by the caller", false, 3, ptr(true)},
		{"integer added by the caller at the limit", false, int64(10), ptr(false)},
		{"text from a query string", false, "3", nil},
		{"number that is not whole", false, 2.5, nil},
		{"absent", false, nil, nil},
	}
	for _, c := range cases {
		for _, suppress := range []bool{false, true} {
			name := fmt.Sprintf("%s/suppress=%v", c.name, suppress)
			t.Run(name, func(t *testing.T) {
				checkSequenceNumber(t, c.withSequence, c.value, c.wantJS, suppress)
			})
		}
	}
}

func ptr(b bool) *bool { return &b }

// checkSequenceNumber processes one request and checks the JSON Builder's
// result against wantJS.
func checkSequenceNumber(t *testing.T, withSequence bool, value any,
	wantJS *bool, suppress bool) {
	t.Helper()
	b := core.NewPipelineBuilder().SetSuppressProcessExceptions(suppress)
	if withSequence {
		b.AddFlowElement(sequence.New(nil))
	}
	p, err := b.AddFlowElement(newDeviceEngine()).
		AddFlowElement(New(nil, nil)).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	if value != nil {
		fd.AddEvidence(sequence.EvidenceSequence, value)
	}
	returned := fd.Process()

	failure := returned
	if suppress {
		if returned != nil {
			t.Fatalf("process returned %v, want any error recorded only", returned)
		}
		for _, fe := range fd.Errors() {
			failure = fe.Err
		}
	}
	data, getErr := fd.Get(DataKey)

	if wantJS == nil {
		if failure == nil {
			t.Fatal("the JSON Builder ran with no integer sequence number")
		}
		var config *core.ConfigurationError
		if !errors.As(failure, &config) {
			t.Errorf("the error %T is not a *core.ConfigurationError", failure)
		}
		if !strings.Contains(failure.Error(), "Sequence Element") {
			t.Errorf("the error %q does not say a Sequence Element is needed",
				failure)
		}
		if getErr == nil {
			t.Errorf("the JSON Builder set its JSON %q", data.(*Data).JSON())
		}
		return
	}

	if failure != nil || getErr != nil {
		t.Fatalf("processing gives %v and the JSON %v, want the JSON",
			failure, getErr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(data.(*Data).JSON()), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, hasJS := result["javascriptProperties"]; hasJS != *wantJS {
		t.Errorf("javascriptProperties written is %v, want %v", hasJS, *wantJS)
	}
}
