package core

import (
	"errors"
	"log/slog"
	"reflect"
	"testing"
)

func TestProcessErrorUnwrap(t *testing.T) {
	e1 := errors.New("one")
	e2 := errors.New("two")
	pe := &ProcessError{Errors: []FlowError{{Err: e1}, {Err: e2}}}
	if !errors.Is(pe, e1) || !errors.Is(pe, e2) {
		t.Fatal("ProcessError should unwrap to each contained error")
	}
	if pe.Error() == "" {
		t.Fatal("empty error string")
	}
}

func TestConfigurationErrorUnwrap(t *testing.T) {
	cause := errors.New("root")
	ce := NewConfigurationError("bad", cause)
	if !errors.Is(ce, cause) {
		t.Fatal("ConfigurationError should unwrap to its cause")
	}
}

func TestNoValueError(t *testing.T) {
	nve := NewNoValueError("no value here", nil)
	if nve.Error() != "no value here" {
		t.Fatalf("unexpected message: %s", nve.Error())
	}
}

func TestAggregateFilterKeys(t *testing.T) {
	f := NewAggregateFilter(
		NewWhitelistFilter("header.user-agent", "query.x"),
		NewWhitelistFilter("header.user-agent", "cookie.y"),
		NewFuncFilter(func(string) bool { return true }), // not enumerable
	)
	keys := f.Keys()
	// De-duplicated union of the enumerable filters; the func filter contributes none.
	want := map[string]bool{"header.user-agent": true, "query.x": true, "cookie.y": true}
	if len(keys) != len(want) {
		t.Fatalf("got %v", keys)
	}
	for _, k := range keys {
		if !want[k] {
			t.Fatalf("unexpected key %q", k)
		}
	}
}

func TestGetEvidenceOrProperty(t *testing.T) {
	// Build a pipeline whose element exposes a property named "platform".
	el := &propElement{BaseElement: NewBaseElement("device", NewWhitelistFilter(), nil)}
	p, _ := NewPipelineBuilder().AddFlowElement(el).Build()
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("query.platform", "from-evidence")
	_ = fd.Process()

	// Element data has "platform" -> should be returned in preference to evidence.
	v, ok, err := GetEvidenceOrProperty(fd, "query.platform")
	if err != nil || !ok {
		t.Fatalf("err=%v ok=%v", err, ok)
	}
	if v != "from-property" {
		t.Fatalf("expected element-data value, got %v", v)
	}

	// A key not produced by any element falls back to evidence.
	fd.AddEvidence("query.other", "ev") // ignored (post-process) but evidence already has platform
	v2, ok2, _ := GetEvidenceOrProperty(fd, "query.missing")
	if ok2 || v2 != nil {
		t.Fatalf("expected miss, got %v %v", v2, ok2)
	}
}

type propElement struct{ *BaseElement }

func (e *propElement) Process(fd FlowData) error {
	d := fd.GetOrAdd("device", func(FlowData) ElementData { return NewBaseElementData() })
	d.Set("platform", "from-property")
	return nil
}

func TestBuildFromConfigurationUnknownBuilder(t *testing.T) {
	r := NewBuilderRegistry()
	_, err := r.BuildFromConfiguration(PipelineOptions{
		Elements: []ElementOptions{{BuilderName: "DoesNotExist"}},
	}, nil)
	var ce *ConfigurationError
	if !errors.As(err, &ce) {
		t.Fatalf("expected ConfigurationError, got %v", err)
	}
}

func TestLoadPipelineOptionsErrors(t *testing.T) {
	if _, err := LoadPipelineOptions("/no/such/file.json"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParallelCollectsChildErrors(t *testing.T) {
	good := &propElement{BaseElement: NewBaseElement("device", NewWhitelistFilter(), nil)}
	bad := newFailingElement(errors.New("child boom"))
	p, _ := NewPipelineBuilder().SetConcurrent(true).AddParallel(good, bad).Build()
	defer p.Close()
	fd := p.CreateFlowData()
	err := fd.Process()
	if err == nil {
		t.Fatal("expected error from failing child")
	}
	if d, e := fd.Get("device"); e != nil {
		t.Fatalf("good child should still have produced data: %v", e)
	} else if v, _ := d.Get("platform"); v != "from-property" {
		t.Fatalf("got %v", v)
	}
}

func TestMultiPipelineMembership(t *testing.T) {
	el := &propElement{BaseElement: NewBaseElement("device", NewWhitelistFilter(), nil)}
	p1, _ := NewPipelineBuilder().AddFlowElement(el).Build()
	p2, _ := NewPipelineBuilder().AddFlowElement(el).Build()
	defer p1.Close()
	defer p2.Close()
	if len(el.Pipelines()) != 2 {
		t.Fatalf("element should track both pipelines, got %d", len(el.Pipelines()))
	}
}

func TestPipelinePropertiesAreCopied(t *testing.T) {
	el := &propElement{BaseElement: NewBaseElement("device", NewWhitelistFilter(), nil)}
	el.SetProperties([]PropertyMetadata{{Name: "platform", Element: el, Type: reflect.TypeOf("")}})
	p, _ := NewPipelineBuilder().AddFlowElement(el).Build()
	defer p.Close()
	props := p.Properties()
	props[0].Name = "mutated"
	if p.Properties()[0].Name != "platform" {
		t.Fatal("Properties() must return a copy")
	}
}

func TestWeightedValueMarshalJSON(t *testing.T) {
	w := NewWeightedValue("london", 32768)
	b, err := w.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if got != `{"RawWeighting":32768,"Value":"london"}` {
		t.Fatalf("unexpected JSON: %s", got)
	}
}

func TestDiscardLoggerBuild(t *testing.T) {
	// Building with an explicit logger should not panic and should be used.
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	p, err := NewPipelineBuilder().SetLogger(logger).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
}
