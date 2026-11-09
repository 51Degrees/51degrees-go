package core

import (
	"log/slog"
	"sort"
	"testing"
)

// funcElement is a Flow Element that runs a function when it processes.
type funcElement struct {
	*BaseElement
	run func(FlowData) error
}

func newFuncElement(key string, filter EvidenceKeyFilter, run func(FlowData) error) *funcElement {
	return &funcElement{BaseElement: NewBaseElement(key, filter, nil), run: run}
}

func (e *funcElement) Process(fd FlowData) error { return e.run(fd) }

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(discard{}, nil)) }

// A Flow Element may add Evidence while it processes. A later Flow Element
// reads the value, and so does the caller once processing is complete. The
// value is added although the adding element does not accept that key.
func TestFlowElementAddsEvidence(t *testing.T) {
	var seenByReader any
	var seenOK bool
	writer := newFuncElement("writer", NewWhitelistFilter("query.other"), func(fd FlowData) error {
		fd.AddEvidence("query.added", "from the writer")
		return nil
	})
	reader := newFuncElement("reader", NewWhitelistFilter("query.added"), func(fd FlowData) error {
		seenByReader, seenOK = fd.Evidence().Get("query.added")
		return nil
	})
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).AddFlowElement(writer).AddFlowElement(reader).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}

	if !seenOK || seenByReader != "from the writer" {
		t.Errorf("later element read %v (present %v), want the added value", seenByReader, seenOK)
	}
	if v, ok := fd.Evidence().Get("query.added"); !ok || v != "from the writer" {
		t.Errorf("caller read %v (present %v), want the added value", v, ok)
	}
}

// An added value replaces the value under the same key, ignoring case.
func TestFlowElementAddedEvidenceReplacesIgnoringCase(t *testing.T) {
	var before any
	replacer := newFuncElement("replacer", nil, func(fd FlowData) error {
		before, _ = fd.Evidence().Get("query.month")
		fd.AddEvidence("QUERY.Month", "april")
		return nil
	})
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).AddFlowElement(replacer).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	fd.AddEvidence("query.month", "march")
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}

	if before != "march" {
		t.Errorf("element read %v before adding, want march", before)
	}
	if v, _ := fd.Evidence().Get("query.month"); v != "april" {
		t.Errorf("value = %v, want april", v)
	}
	if keys := fd.Evidence().Keys(); len(keys) != 1 {
		t.Errorf("keys = %v, want one", keys)
	}
}

// A Flow Element in a parallel group must not add Evidence, as the other
// members read it at the same time. The value is not added, and a Flow Element
// that runs after the group can add one.
func TestParallelGroupAddsNoEvidence(t *testing.T) {
	var reads []any
	var readsDone = make(chan struct{})
	writer := newFuncElement("parallel-writer", nil, func(fd FlowData) error {
		fd.AddEvidence("query.from-group", "must not be added")
		return nil
	})
	reader := newFuncElement("parallel-reader", nil, func(fd FlowData) error {
		for i := 0; i < 100; i++ {
			v, _ := fd.Evidence().Get("query.from-group")
			reads = append(reads, v)
		}
		close(readsDone)
		return nil
	})
	after := newFuncElement("after-group", nil, func(fd FlowData) error {
		fd.AddEvidence("query.after-group", "added after the group")
		return nil
	})
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).SetConcurrent(true).
		AddParallel(writer, reader).AddFlowElement(after).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	<-readsDone

	if v, ok := fd.Evidence().Get("query.from-group"); ok {
		t.Errorf("a parallel group element added %v", v)
	}
	for _, v := range reads {
		if v != nil {
			t.Fatalf("the reader saw %v from the group", v)
		}
	}
	if v, ok := fd.Evidence().Get("query.after-group"); !ok || v != "added after the group" {
		t.Errorf("value from the element after the group = %v (present %v)", v, ok)
	}
	keys := fd.Evidence().Keys()
	sort.Strings(keys)
	if len(keys) != 1 || keys[0] != "query.after-group" {
		t.Errorf("keys = %v, want only query.after-group", keys)
	}
}

// The caller cannot add Evidence once processing is complete.
func TestCallerAddsNoEvidenceAfterProcessing(t *testing.T) {
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).AddFlowElement(
		newFuncElement("noop", nil, func(FlowData) error { return nil })).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("query.month", "march")
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	fd.AddEvidence("query.month", "april")
	fd.AddEvidence("query.new", "value")
	if v, _ := fd.Evidence().Get("query.month"); v != "march" {
		t.Errorf("value = %v, want march", v)
	}
	if _, ok := fd.Evidence().Get("query.new"); ok {
		t.Error("a key was added after processing")
	}
}

// SuppliedEvidence holds what the caller supplied, without the values that
// Flow Elements added or replaced while processing.
func TestSuppliedEvidenceLeavesOutAddedValues(t *testing.T) {
	adder := newFuncElement("adder", nil, func(fd FlowData) error {
		fd.AddEvidence("query.added", "by the element")
		fd.AddEvidence("query.month", "april")
		return nil
	})
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).AddFlowElement(adder).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	fd.AddEvidence("query.month", "march")
	if got := SuppliedEvidence(fd); len(got) != 1 || got["query.month"] != "march" {
		t.Errorf("before processing, supplied = %v", got)
	}
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}

	got := SuppliedEvidence(fd)
	if len(got) != 1 || got["query.month"] != "march" {
		t.Errorf("supplied = %v, want only query.month=march", got)
	}
	if v, _ := fd.Evidence().Get("query.month"); v != "april" {
		t.Errorf("Evidence query.month = %v, want april", v)
	}
}
