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

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
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
	// A GUID-shaped session id is preserved; the sequence is incremented.
	const guid = "12345678-1234-4234-8234-1234567890ab"
	data := run(t, map[string]any{
		EvidenceSessionID: guid,
		EvidenceSequence:  "4",
	})
	if sid, _ := data.Get(SessionIDProperty); sid != guid {
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

// The specification asks for both outputs in Evidence as well as in Element
// Data, so that later Flow Elements and the caller read them from Evidence.
func TestOutputsWrittenToEvidence(t *testing.T) {
	const guid = "12345678-1234-4234-8234-1234567890ab"
	p, err := core.NewPipelineBuilder().AddFlowElement(New(nil)).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence(EvidenceSessionID, guid)
	fd.AddEvidence(EvidenceSequence, "4")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	if v, _ := fd.Evidence().Get(EvidenceSessionID); v != guid {
		t.Errorf("Evidence %s = %v, want %s", EvidenceSessionID, v, guid)
	}
	if v, _ := fd.Evidence().Get(EvidenceSequence); v != 5 {
		t.Errorf("Evidence %s = %v, want 5", EvidenceSequence, v)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if v, _ := data.Get(SequenceProperty); v != 5 {
		t.Errorf("Element Data sequence = %v, want 5", v)
	}
}

func TestNewSessionWrittenToEvidence(t *testing.T) {
	p, err := core.NewPipelineBuilder().AddFlowElement(New(nil)).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	sid, _ := data.Get(SessionIDProperty)
	if v, ok := fd.Evidence().Get(EvidenceSessionID); !ok || v != sid {
		t.Errorf("Evidence session id = %v (present %v), want %v", v, ok, sid)
	}
	if v, ok := fd.Evidence().Get(EvidenceSequence); !ok || v != 1 {
		t.Errorf("Evidence sequence = %v (present %v), want 1", v, ok)
	}
}

// The session id and the sequence are taken independently.
func TestSessionIDAndSequenceAreIndependent(t *testing.T) {
	const supplied = "any non-empty text"
	cases := []struct {
		name     string
		evidence map[string]any
		wantID   string // empty means a new id
		wantSeq  int
	}{
		{"sequence with no session id counts up with a new id",
			map[string]any{EvidenceSequence: "4"}, "", 5},
		{"session id with no sequence gives 1",
			map[string]any{EvidenceSessionID: supplied}, supplied, 1},
		{"an id of 0 is kept", map[string]any{EvidenceSessionID: "0"}, "0", 1},
		{"any non-empty string is kept as the id",
			map[string]any{EvidenceSessionID: `"; x="`}, `"; x="`, 1},
		{"an empty id is absent", map[string]any{EvidenceSessionID: ""}, "", 1},
		{"an id that is not a string is absent", map[string]any{EvidenceSessionID: 7}, "", 1},
		{"sequence 0 gives 1", map[string]any{EvidenceSequence: "0"}, "", 1},
		{"sequence as an int", map[string]any{EvidenceSequence: 2}, "", 3},
		{"sequence as a whole float", map[string]any{EvidenceSequence: float64(2)}, "", 3},
		{"sequence as a fraction is absent", map[string]any{EvidenceSequence: 2.5}, "", 1},
		{"sequence as negative text is absent", map[string]any{EvidenceSequence: "-1"}, "", 1},
		{"sequence as negative number is absent", map[string]any{EvidenceSequence: -1}, "", 1},
		{"sequence with a sign is absent", map[string]any{EvidenceSequence: "+1"}, "", 1},
		{"sequence with spaces is absent", map[string]any{EvidenceSequence: " 1"}, "", 1},
		{"sequence as words is absent", map[string]any{EvidenceSequence: "one"}, "", 1},
		{"sequence as empty text is absent", map[string]any{EvidenceSequence: ""}, "", 1},
		{"sequence out of range is absent",
			map[string]any{EvidenceSequence: "99999999999999999999"}, "", 1},
		{"the largest sequence that can be counted up",
			map[string]any{EvidenceSequence: "2147483646"}, "", 2147483647},
		{"one more than that is absent",
			map[string]any{EvidenceSequence: "2147483647"}, "", 1},
		{"both supplied",
			map[string]any{EvidenceSessionID: supplied, EvidenceSequence: "9"}, supplied, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := core.NewPipelineBuilder().AddFlowElement(New(nil)).Build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			defer p.Close()
			fd := p.CreateFlowData()
			for k, v := range c.evidence {
				fd.AddEvidence(k, v)
			}
			if err := fd.Process(); err != nil {
				t.Fatalf("process: %v", err)
			}
			data, _ := fd.Get(DataKey)
			sid, _ := data.Get(SessionIDProperty)
			seq, _ := data.Get(SequenceProperty)
			if c.wantID != "" && sid != c.wantID {
				t.Errorf("session id = %v, want %q", sid, c.wantID)
			}
			if c.wantID == "" && (sid == "" || sid == nil || len(sid.(string)) != 36) {
				t.Errorf("session id = %v, want a new GUID", sid)
			}
			if seq != c.wantSeq {
				t.Errorf("sequence = %v, want %d", seq, c.wantSeq)
			}
			// Evidence holds the same values as the Element Data.
			if v, _ := fd.Evidence().Get(EvidenceSessionID); v != sid {
				t.Errorf("Evidence session id = %v, Element Data %v", v, sid)
			}
			if v, _ := fd.Evidence().Get(EvidenceSequence); v != seq {
				t.Errorf("Evidence sequence = %v, Element Data %v", v, seq)
			}
		})
	}
}
