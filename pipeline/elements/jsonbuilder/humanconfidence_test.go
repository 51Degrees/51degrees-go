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
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/sequence"
)

// The JSON Builder's output never holds a sealed Human Confidence result or
// the page id sent with it, because it is built from Element Data, and the
// only Evidence it reads is the sequence.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#what-a-package-never-exposes
func TestTheOutputHoldsNoSealedHumanConfidence(t *testing.T) {
	p, err := core.NewPipelineBuilder().AddFlowElement(sequence.New(nil)).
		AddFlowElement(New(nil, nil)).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("query.humanconfidence", "made-up-sealed-value")
	fd.AddEvidence("query.humanconfidencepage", "made-up-page-id")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("get json: %v", err)
	}
	out := data.(*Data).JSON()
	for _, value := range []string{"made-up-sealed-value", "made-up-page-id"} {
		if strings.Contains(out, value) {
			t.Errorf("the JSON holds %q: %s", value, out)
		}
	}
}
