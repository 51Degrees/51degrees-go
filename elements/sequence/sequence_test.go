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

package sequence

import (
	"testing"

	"github.com/51Degrees/pipeline-go/core"
)

func run(t *testing.T, evidence map[string]any) core.ElementData {
	t.Helper()
	p, err := core.NewPipelineBuilder().AddFlowElement(New(nil)).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	fd := p.CreateFlowData()
	for k, v := range evidence {
		fd.AddEvidence(k, v)
	}
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	return data
}

func TestNewSession(t *testing.T) {
	data := run(t, nil)
	sid, _ := data.Get(SessionIDProperty)
	if s, ok := sid.(string); !ok || len(s) != 36 {
		t.Fatalf("expected a GUID session id, got %v", sid)
	}
	seq, _ := data.Get(SequenceProperty)
	if seq != 1 {
		t.Fatalf("expected sequence 1, got %v", seq)
	}
}

func TestExistingSession(t *testing.T) {
	data := run(t, map[string]any{
		EvidenceSessionID: "abc-123",
		EvidenceSequence:  "4",
	})
	if sid, _ := data.Get(SessionIDProperty); sid != "abc-123" {
		t.Fatalf("session id should be preserved, got %v", sid)
	}
	if seq, _ := data.Get(SequenceProperty); seq != 5 {
		t.Fatalf("expected sequence incremented to 5, got %v", seq)
	}
}

func TestSequenceFromInt(t *testing.T) {
	data := run(t, map[string]any{EvidenceSequence: 2})
	if seq, _ := data.Get(SequenceProperty); seq != 3 {
		t.Fatalf("expected 3, got %v", seq)
	}
}
