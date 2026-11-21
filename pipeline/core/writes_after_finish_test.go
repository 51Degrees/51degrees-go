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
	"errors"
	"strings"
	"testing"
)

// panicOf runs f and gives what it panicked with, or nil.
func panicOf(f func()) (r any) {
	defer func() { r = recover() }()
	f()
	return nil
}

// wantFinishedPanic fails t unless f panics with an error that says
// processing has finished.
func wantFinishedPanic(t *testing.T, what string, f func()) {
	t.Helper()
	r := panicOf(f)
	err, ok := r.(error)
	if !ok {
		t.Errorf("%s panicked with %v, want an error", what, r)
		return
	}
	if !strings.Contains(err.Error(), "finished processing") {
		t.Errorf("%s panicked with %q, want an error saying processing "+
			"has finished", what, err)
	}
}

// writerElement adds Element Data under its key and sets a value in it.
func writerElement(key string) *funcElement {
	return newFuncElement(key, nil, func(fd FlowData) error {
		fd.GetOrAdd(key, func(FlowData) ElementData {
			return NewBaseElementData()
		}).Set("value", key)
		return nil
	})
}

func processed(t *testing.T, b *PipelineBuilder) FlowData {
	t.Helper()
	p, err := b.SetLogger(quietLogger()).Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	fd := p.CreateFlowData()
	fd.AddEvidence("query.month", "march")
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	return fd
}

// A write to a finished Flow Data panics, which is how Go fails a call that
// returns nothing.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/thread-safety.md?plain=1#L115-L118
func TestAddingToAFinishedFlowDataPanics(t *testing.T) {
	fd := processed(t, NewPipelineBuilder().AddFlowElement(writerElement("a")))
	wantFinishedPanic(t, "AddEvidence", func() {
		fd.AddEvidence("query.month", "april")
	})
	wantFinishedPanic(t, "AddEvidenceMap", func() {
		fd.AddEvidenceMap(map[string]any{"query.new": "value"})
	})
	wantFinishedPanic(t, "GetOrAdd of a new key", func() {
		fd.GetOrAdd("b", func(FlowData) ElementData { return NewBaseElementData() })
	})
	wantFinishedPanic(t, "AddError", func() {
		fd.AddError(errors.New("late"), nil)
	})
	if v, _ := fd.Evidence().Get("query.month"); v != "march" {
		t.Errorf("query.month = %v, want march", v)
	}
	if _, ok := fd.Evidence().Get("query.new"); ok {
		t.Error("a key was added after processing")
	}
	if _, err := fd.Get("b"); err == nil {
		t.Error("Element Data was added after processing")
	}
	if fd.HasErrors() {
		t.Errorf("an error was added after processing: %v", fd.Errors())
	}
	// GetOrAdd of a key already held only reads it.
	if r := panicOf(func() {
		fd.GetOrAdd("a", func(FlowData) ElementData { return NewBaseElementData() })
	}); r != nil {
		t.Errorf("GetOrAdd of a key already held panicked with %v", r)
	}
}

// A write to Element Data once the Flow Element that writes it has finished
// panics.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/thread-safety.md?plain=1#L141-L149
func TestWritingElementDataAfterItsElementFinishedPanics(t *testing.T) {
	fd := processed(t, NewPipelineBuilder().AddFlowElement(writerElement("a")))
	d, err := fd.Get("a")
	if err != nil {
		t.Fatal(err)
	}
	wantFinishedPanic(t, "Set", func() { d.Set("value", "late") })
	if v, _ := d.Get("value"); v != "a" {
		t.Errorf("value = %v, want a", v)
	}
}

// A later Flow Element cannot write the Element Data of one that has
// finished, and the panic is recorded as that Flow Element's error.
func TestALaterElementCannotWriteAnEarlierElementsData(t *testing.T) {
	later := newFuncElement("later", nil, func(fd FlowData) error {
		d, err := fd.Get("a")
		if err != nil {
			return err
		}
		d.Set("value", "from later")
		return nil
	})
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).
		AddFlowElement(writerElement("a")).AddFlowElement(later).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err == nil ||
		!strings.Contains(err.Error(), "finished processing") {
		t.Errorf("Process gave %v, want the later element's write to fail", err)
	}
	d, _ := fd.Get("a")
	if v, _ := d.Get("value"); v != "a" {
		t.Errorf("value = %v, want a", v)
	}
}

// A Flow Element writes its own Element Data as often as it likes while it
// processes, and a second Flow Element with the same key shares and writes it.
func TestElementsWriteTheirOwnDataWhileTheyProcess(t *testing.T) {
	first := newFuncElement("shared", nil, func(fd FlowData) error {
		d := fd.GetOrAdd("shared", func(FlowData) ElementData {
			return NewBaseElementData()
		})
		d.Set("one", 1)
		d.Set("one", 2)
		return nil
	})
	second := newFuncElement("shared", nil, func(fd FlowData) error {
		fd.GetOrAdd("shared", nil).Set("two", 2)
		return nil
	})
	fd := processed(t, NewPipelineBuilder().AddFlowElement(first).
		AddFlowElement(second))
	d, _ := fd.Get("shared")
	if got := d.AsMap(); got["one"] != 2 || got["two"] != 2 {
		t.Errorf("shared data = %v, want one and two both 2", got)
	}
	wantFinishedPanic(t, "Set", func() { d.Set("three", 3) })
}

// The members of a parallel group write their own Element Data while the
// group runs, and none of it can be written once processing has finished.
func TestParallelElementsDataIsReadOnlyOnceProcessed(t *testing.T) {
	fd := processed(t, NewPipelineBuilder().
		AddParallel(writerElement("a"), writerElement("b"), writerElement("c")))
	for _, key := range []string{"a", "b", "c"} {
		d, err := fd.Get(key)
		if err != nil {
			t.Fatal(err)
		}
		if v, _ := d.Get("value"); v != key {
			t.Errorf("%s value = %v, want %s", key, v, key)
		}
		wantFinishedPanic(t, key+" Set", func() { d.Set("value", "late") })
	}
}

// Element Data the caller adds before processing can be written until
// processing has finished.
func TestElementDataTheCallerAddsIsReadOnlyOnceProcessed(t *testing.T) {
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).
		AddFlowElement(writerElement("a")).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	mine := fd.GetOrAdd("mine", func(FlowData) ElementData {
		return NewBaseElementData()
	})
	mine.Set("before", true)
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	wantFinishedPanic(t, "Set", func() { mine.Set("after", true) })
}
