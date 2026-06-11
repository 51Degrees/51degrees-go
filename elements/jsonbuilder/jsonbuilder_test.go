package jsonbuilder

import (
	"encoding/json"
	"testing"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/elements/sequence"
	"github.com/51Degrees/pipeline-go/engines"
)

// deviceEngine is a fake engine producing a mix of value types for the JSON
// Builder to serialize.
type deviceEngine struct{ *core.BaseElement }

func newDeviceEngine() *deviceEngine {
	e := &deviceEngine{BaseElement: core.NewBaseElement("device", core.NewWhitelistFilter(), nil)}
	e.SetProperties([]core.PropertyMetadata{
		{Name: "ismobile", Element: e, Available: true},
		{Name: "zipcode", Element: e, Available: true},
		{Name: "javascript", Element: e, Available: true, DelayExecution: true},
	})
	return e
}

func (e *deviceEngine) Process(fd core.FlowData) error {
	d := fd.GetOrAdd("device", func(core.FlowData) core.ElementData { return core.NewBaseElementData() })
	d.Set("ismobile", true)
	d.Set("zipcode", engines.NoValue[string]("requires client-side evidence"))
	d.Set("javascript", core.JavaScript("navigator.geolocation"))
	return nil
}

// excludedEngine writes under an excluded data key.
type excludedEngine struct{ *core.BaseElement }

func newExcludedEngine() *excludedEngine {
	e := &excludedEngine{BaseElement: core.NewBaseElement("cloud-response", core.NewWhitelistFilter(), nil)}
	return e
}

func (e *excludedEngine) Process(fd core.FlowData) error {
	d := fd.GetOrAdd("cloud-response", func(core.FlowData) core.ElementData { return core.NewBaseElementData() })
	d.Set("secret", "should-not-appear")
	return nil
}

func buildJSON(t *testing.T, elements ...core.FlowElement) map[string]any {
	t.Helper()
	b := core.NewPipelineBuilder().AddFlowElement(sequence.New(nil))
	for _, e := range elements {
		b.AddFlowElement(e)
	}
	b.AddFlowElement(New(nil, nil))
	p, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("get json: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(data.(*Data).JSON()), &result); err != nil {
		t.Fatalf("invalid JSON produced: %v", err)
	}
	return result
}

func TestJSONBuilderBasic(t *testing.T) {
	result := buildJSON(t, newDeviceEngine())

	device, ok := result["device"].(map[string]any)
	if !ok {
		t.Fatalf("missing device object: %v", result)
	}
	if device["ismobile"] != true {
		t.Fatalf("ismobile = %v", device["ismobile"])
	}
	// Null value -> null plus nullreason.
	if device["zipcode"] != nil {
		t.Fatalf("zipcode should be null, got %v", device["zipcode"])
	}
	if device["zipcodenullreason"] != "requires client-side evidence" {
		t.Fatalf("zipcodenullreason = %v", device["zipcodenullreason"])
	}
	// Delay execution meta.
	if device["javascriptdelayexecution"] != true {
		t.Fatalf("expected javascriptdelayexecution true, got %v", device["javascriptdelayexecution"])
	}
	// JavaScript property listed.
	jsProps, ok := result["javascriptProperties"].([]any)
	if !ok {
		t.Fatalf("missing javascriptProperties: %v", result["javascriptProperties"])
	}
	found := false
	for _, p := range jsProps {
		if p == "device.javascript" {
			found = true
		}
	}
	if !found {
		t.Fatalf("device.javascript not listed: %v", jsProps)
	}
}

func TestJSONBuilderExclusion(t *testing.T) {
	result := buildJSON(t, newDeviceEngine(), newExcludedEngine())
	if _, present := result["cloud-response"]; present {
		t.Fatal("cloud-response element should be excluded from JSON")
	}
}

func TestJSONBuilderSetProperties(t *testing.T) {
	// Restrict to ismobile only; javascript must still be included.
	b := core.NewPipelineBuilder().
		AddFlowElement(sequence.New(nil)).
		AddFlowElement(newDeviceEngine()).
		AddFlowElement(New(nil, []string{"device.ismobile"}))
	p, _ := b.Build()
	defer p.Close()
	fd := p.CreateFlowData()
	_ = fd.Process()
	data, _ := fd.Get(DataKey)
	var result map[string]any
	_ = json.Unmarshal([]byte(data.(*Data).JSON()), &result)
	device := result["device"].(map[string]any)
	if _, ok := device["ismobile"]; !ok {
		t.Fatal("ismobile should be included")
	}
	if _, ok := device["zipcode"]; ok {
		t.Fatal("zipcode should be excluded by SetProperties")
	}
	if _, ok := device["javascript"]; !ok {
		t.Fatal("javascript must always be included")
	}
}
