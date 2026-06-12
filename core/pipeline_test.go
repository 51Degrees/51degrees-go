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
	"reflect"
	"sync"
	"testing"
)

// starSignData is a sample ElementData produced by the test element below.
type starSignData struct {
	*BaseElementData
}

func newStarSignData() *starSignData { return &starSignData{BaseElementData: NewBaseElementData()} }

// starSignElement is a minimal Flow Element used to exercise the Pipeline. It
// reads a month from evidence and writes a (fake) star sign.
type starSignElement struct {
	*BaseElement
}

func newStarSignElement() *starSignElement {
	e := &starSignElement{BaseElement: NewBaseElement("starsign", NewWhitelistFilter("query.month"), nil)}
	e.SetProperties([]PropertyMetadata{
		{Name: "starsign", Element: e, Category: "Astrology", Type: reflect.TypeOf(""), Available: true},
	})
	return e
}

func (e *starSignElement) Process(fd FlowData) error {
	month, ok := fd.Evidence().Get("query.month")
	data := fd.GetOrAdd(e.DataKey(), func(FlowData) ElementData { return newStarSignData() })
	if !ok {
		data.Set("starsign", "Unknown")
		return nil
	}
	switch month {
	case "march":
		data.Set("starsign", "Aries")
	default:
		data.Set("starsign", "Unknown")
	}
	return nil
}

// failingElement always returns an error during processing.
type failingElement struct {
	*BaseElement
	err error
}

func newFailingElement(err error) *failingElement {
	return &failingElement{BaseElement: NewBaseElement("failing", nil, nil), err: err}
}

func (e *failingElement) Process(fd FlowData) error { return e.err }

func TestPipelineProcessAndAccess(t *testing.T) {
	el := newStarSignElement()
	p, err := NewPipelineBuilder().AddFlowElement(el).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	fd.AddEvidence("query.month", "march")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	// Access by string key.
	data, err := fd.Get("starsign")
	if err != nil {
		t.Fatalf("get by key: %v", err)
	}
	if v, _ := data.Get("starsign"); v != "Aries" {
		t.Fatalf("got %v, want Aries", v)
	}

	// Access by element instance.
	d2, err := GetFromElement[*starSignData](fd, el)
	if err != nil {
		t.Fatalf("GetFromElement: %v", err)
	}
	if v, _ := GetPropertyAs[string](d2, "starsign"); v != "Aries" {
		t.Fatalf("typed property: got %v", v)
	}

	// Access by type.
	d3, err := Get[*starSignData](fd)
	if err != nil {
		t.Fatalf("Get by type: %v", err)
	}
	if v, _ := d3.Get("starsign"); v != "Aries" {
		t.Fatalf("type access: got %v", v)
	}
}

func TestProcessOnce(t *testing.T) {
	p, _ := NewPipelineBuilder().AddFlowElement(newStarSignElement()).Build()
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatalf("first process: %v", err)
	}
	if err := fd.Process(); !errors.Is(err, ErrAlreadyProcessed) {
		t.Fatalf("want ErrAlreadyProcessed, got %v", err)
	}
}

func TestAccessBeforeProcess(t *testing.T) {
	p, _ := NewPipelineBuilder().AddFlowElement(newStarSignElement()).Build()
	defer p.Close()
	fd := p.CreateFlowData()
	if _, err := fd.Get("starsign"); !errors.Is(err, ErrNotProcessed) {
		t.Fatalf("want ErrNotProcessed, got %v", err)
	}
}

func TestProcessExceptionPropagation(t *testing.T) {
	myErr := errors.New("boom")
	p, _ := NewPipelineBuilder().AddFlowElement(newFailingElement(myErr)).Build()
	defer p.Close()
	fd := p.CreateFlowData()
	err := fd.Process()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, myErr) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
	if len(fd.Errors()) != 1 {
		t.Fatalf("expected 1 collected error, got %d", len(fd.Errors()))
	}
}

func TestSuppressProcessExceptions(t *testing.T) {
	p, _ := NewPipelineBuilder().
		AddFlowElement(newFailingElement(errors.New("boom"))).
		SetSuppressProcessExceptions(true).
		Build()
	defer p.Close()
	if !p.IsSuppressProcessExceptions() {
		t.Fatal("expected suppression enabled")
	}
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatalf("expected nil error when suppressed, got %v", err)
	}
	if !fd.HasErrors() {
		t.Fatal("errors should still be collected when suppressed")
	}
}

func TestPanicInElementIsCaught(t *testing.T) {
	p, _ := NewPipelineBuilder().AddFlowElement(&panicElement{NewBaseElement("panic", nil, nil)}).Build()
	defer p.Close()
	fd := p.CreateFlowData()
	err := fd.Process()
	if err == nil {
		t.Fatal("expected error from panic")
	}
}

type panicElement struct{ *BaseElement }

func (e *panicElement) Process(fd FlowData) error { panic("kaboom") }

func TestPipelineProperties(t *testing.T) {
	el := newStarSignElement()
	p, _ := NewPipelineBuilder().AddFlowElement(el).Build()
	defer p.Close()
	props := p.Properties()
	if len(props) != 1 || props[0].Name != "starsign" {
		t.Fatalf("unexpected properties: %+v", props)
	}
	if p.GetElement("StarSign") == nil {
		t.Fatal("GetElement should be case-insensitive")
	}
}

func TestParallelElements(t *testing.T) {
	// Two independent elements run in parallel; requires concurrent pipeline.
	a := newStarSignElement()
	b := &secondElement{NewBaseElement("second", nil, nil)}
	p, _ := NewPipelineBuilder().SetConcurrent(true).AddParallel(a, b).Build()
	defer p.Close()

	if !p.IsConcurrent() {
		t.Fatal("expected concurrent pipeline")
	}
	fd := p.CreateFlowData()
	fd.AddEvidence("query.month", "march")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	if d, err := fd.Get("starsign"); err != nil {
		t.Fatalf("starsign: %v", err)
	} else if v, _ := d.Get("starsign"); v != "Aries" {
		t.Fatalf("got %v", v)
	}
	if d, err := fd.Get("second"); err != nil {
		t.Fatalf("second: %v", err)
	} else if v, _ := d.Get("value"); v != 99 {
		t.Fatalf("got %v", v)
	}
}

type secondElement struct{ *BaseElement }

func (e *secondElement) Process(fd FlowData) error {
	d := fd.GetOrAdd(e.DataKey(), func(FlowData) ElementData { return NewBaseElementData() })
	d.Set("value", 99)
	return nil
}

func TestConcurrentProcessing(t *testing.T) {
	// Same Pipeline, many Flow Data instances processed concurrently.
	p, _ := NewPipelineBuilder().AddFlowElement(newStarSignElement()).Build()
	defer p.Close()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fd := p.CreateFlowData()
			fd.AddEvidence("query.month", "march")
			if err := fd.Process(); err != nil {
				t.Errorf("process: %v", err)
			}
		}()
	}
	wg.Wait()
}
