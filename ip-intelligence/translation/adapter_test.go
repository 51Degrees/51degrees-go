/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2025 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
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

package translation

import (
	"errors"
	"testing"

	"github.com/51Degrees/ip-intelligence-go/v4/ipi_interop"
	"github.com/51Degrees/pipeline-go/core"
)

// fakeProcessor is a stand-in for the on-premise engine.
type fakeProcessor struct {
	values ipi_interop.Values
	err    error
}

func (f fakeProcessor) Process(string) (ipi_interop.Values, error) {
	return f.values, f.err
}

func TestIpiEngineElementStoresWeightedCodes(t *testing.T) {
	values := ipi_interop.Values{}
	values.AppendWithWeight(PropertyCountryCodesGeographical, "FR", 0.6)
	values.AppendWithWeight(PropertyCountryCodesGeographical, "DE", 0.4)
	values.AppendWithWeight(PropertyCountryCodesPopulation, "US", 1.0)

	p, err := core.NewPipelineBuilder().
		AddFlowElement(NewIpiEngineElement(fakeProcessor{values: values}, nil)).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	fd.AddEvidence(EvidenceServerClientIP, "1.2.3.4")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	d, err := fd.Get(IPIElementDataKey)
	if err != nil {
		t.Fatalf("get ip data: %v", err)
	}
	geo, _ := d.Get(PropertyCountryCodesGeographical)
	codes, _ := geo.([]core.WeightedValue[string])
	if len(codes) != 2 {
		t.Fatalf("expected 2 geo codes, got %d", len(codes))
	}
	// The 0..1 confidence is scaled to the pipeline's 16-bit raw weighting, so
	// assert the values and that the ordering (weight) is carried through.
	if codes[0].Value() != "FR" || codes[1].Value() != "DE" {
		t.Fatalf("geo codes = (%q, %q), want (FR, DE)",
			codes[0].Value(), codes[1].Value())
	}
	if codes[0].RawWeighting() <= codes[1].RawWeighting() {
		t.Fatalf("geo weights not carried in order: FR=%d, DE=%d",
			codes[0].RawWeighting(), codes[1].RawWeighting())
	}
}

func TestIpiEngineElementNoClientIP(t *testing.T) {
	p, err := core.NewPipelineBuilder().
		SetSuppressProcessExceptions(true).
		AddFlowElement(NewIpiEngineElement(fakeProcessor{}, nil)).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	_ = fd.Process()
	if !fd.HasErrors() {
		t.Fatal("expected a flow error when no client IP is supplied")
	}
	d, err := fd.Get(IPIElementDataKey)
	if err != nil {
		t.Fatalf("get ip data: %v", err)
	}
	geo, _ := d.Get(PropertyCountryCodesGeographical)
	if codes, _ := geo.([]core.WeightedValue[string]); len(codes) != 0 {
		t.Fatalf("expected empty codes, got %d", len(codes))
	}
}

func TestIpiEngineElementProcessError(t *testing.T) {
	p, err := core.NewPipelineBuilder().
		SetSuppressProcessExceptions(true).
		AddFlowElement(NewIpiEngineElement(
			fakeProcessor{err: errors.New("boom")}, nil)).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	fd.AddEvidence(EvidenceQueryClientIP, "8.8.8.8")
	_ = fd.Process()
	if !fd.HasErrors() {
		t.Fatal("expected a flow error when the engine fails")
	}
}
