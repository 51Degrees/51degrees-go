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

package core

import (
	"bytes"
	"log/slog"
	"slices"
	"sync"
	"testing"
)

// recordedLog keeps what a logger writes.
type recordedLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (r *recordedLog) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.Write(p)
}

func (r *recordedLog) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.String()
}

// A member of a parallel group that adds Evidence sees it in its own view of
// the Flow Data, and nothing else does. The group adds nothing on the
// member's behalf, so the value is dropped when the group finishes, with no
// log and no error.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/thread-safety.md?plain=1#L133-L139
// and https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/advanced-features/parallel-processing.md?plain=1#L16-L18
func TestEvidenceAGroupMemberAddsStaysInItsOwnView(t *testing.T) {
	var own, ownFromMap any
	var ownKeys []string
	added := make(chan struct{})
	seenByOther := make(chan any, 1)
	writer := newFuncElement("writer", nil, func(fd FlowData) error {
		fd.AddEvidence("query.from-member", "mine")
		fd.AddEvidenceMap(map[string]any{"query.from-map": "mine too"})
		own, _ = fd.Evidence().Get("Query.From-Member")
		ownFromMap = fd.Evidence().AsMap()["query.from-map"]
		ownKeys = fd.Evidence().Keys()
		close(added)
		return nil
	})
	other := newFuncElement("other", nil, func(fd FlowData) error {
		<-added
		v, _ := fd.Evidence().Get("query.from-member")
		seenByOther <- v
		return nil
	})
	var seenAfter any
	after := newFuncElement("after", nil, func(fd FlowData) error {
		seenAfter, _ = fd.Evidence().Get("query.from-member")
		return nil
	})
	log := &recordedLog{}
	p, err := NewPipelineBuilder().
		SetLogger(slog.New(slog.NewTextHandler(log, nil))).
		AddParallel(writer, other).AddFlowElement(after).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("query.supplied", "by the caller")
	if err := fd.Process(); err != nil {
		t.Fatalf("Process gave %v, want nil", err)
	}

	if own != "mine" || ownFromMap != "mine too" {
		t.Errorf("the member read back %v and %v, want mine and mine too",
			own, ownFromMap)
	}
	slices.Sort(ownKeys)
	want := []string{"query.from-map", "query.from-member", "query.supplied"}
	if !slices.Equal(ownKeys, want) {
		t.Errorf("the member's keys are %v, want %v", ownKeys, want)
	}
	if v := <-seenByOther; v != nil {
		t.Errorf("the other member saw %v", v)
	}
	if seenAfter != nil {
		t.Errorf("the element after the group saw %v", seenAfter)
	}
	if v, ok := fd.Evidence().Get("query.from-member"); ok {
		t.Errorf("the Flow Data kept %v once the group finished", v)
	}
	if fd.HasErrors() {
		t.Errorf("errors were recorded: %v", fd.Errors())
	}
	if s := log.String(); s != "" {
		t.Errorf("something was logged: %s", s)
	}
}
