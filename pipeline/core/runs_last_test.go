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
	"log/slog"
	"strings"
	"testing"
)

// lastElement is a Flow Element that must be the last Flow Element of a
// Pipeline, outside any parallel group, as the usage sharing element must.
type lastElement struct{ *BaseElement }

func newLastElement() *lastElement {
	return &lastElement{NewBaseElement("last", nil, nil)}
}

func (e *lastElement) Process(FlowData) error { return nil }

func (e *lastElement) RunsLast() string { return "the test element" }

// isRefusedForItsPlace reports whether err is the error that says an element
// must be last and outside any parallel group.
func isRefusedForItsPlace(err error) bool {
	return err != nil &&
		strings.Contains(err.Error(), "must be the last Flow Element") &&
		strings.Contains(err.Error(), "outside any parallel group")
}

// A builder refuses a Pipeline that puts an element that must run last
// anywhere but last, or in a parallel group, and builds one that puts it
// last.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/pipeline-elements/usage-sharing-element.md?plain=1#L106-L112
func TestAnElementThatMustRunLastIsRefusedAnywhereElse(t *testing.T) {
	other := func() FlowElement {
		return newFuncElement("other", nil, func(FlowData) error { return nil })
	}
	for _, c := range []struct {
		name    string
		builder *PipelineBuilder
	}{
		{"first", NewPipelineBuilder().AddFlowElement(newLastElement()).
			AddFlowElement(other())},
		{"in a parallel group", NewPipelineBuilder().
			AddParallel(other(), newLastElement())},
		{"in a parallel group at the end", NewPipelineBuilder().
			AddFlowElement(other()).AddParallel(newLastElement())},
	} {
		t.Run(c.name, func(t *testing.T) {
			p, err := c.builder.SetLogger(quietLogger()).Build()
			if err == nil {
				p.Close()
				t.Fatal("the Pipeline was built")
			}
			if !isRefusedForItsPlace(err) {
				t.Errorf("the error %q does not say the element must be the "+
					"last Flow Element and outside any parallel group", err)
			}
		})
	}
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).
		AddFlowElement(other()).AddFlowElement(newLastElement()).Build()
	if err != nil {
		t.Fatalf("a Pipeline with the element last was refused: %v", err)
	}
	p.Close()
}

// A Pipeline built from a configuration is refused in the same way.
func TestAnElementThatMustRunLastIsRefusedAnywhereElseFromAConfiguration(t *testing.T) {
	registry := NewBuilderRegistry()
	registry.Register("Last", func(map[string]any, *slog.Logger) (FlowElement, error) {
		return newLastElement(), nil
	})
	registry.Register("Other", func(map[string]any, *slog.Logger) (FlowElement, error) {
		return newFuncElement("other", nil, func(FlowData) error { return nil }), nil
	})
	p, err := registry.BuildFromConfiguration(PipelineOptions{Elements: []ElementOptions{
		{BuilderName: "Last"}, {BuilderName: "Other"},
	}}, quietLogger())
	if err == nil {
		p.Close()
		t.Fatal("the Pipeline was built")
	}
	if !isRefusedForItsPlace(err) {
		t.Errorf("the error %q does not say the element must be the last "+
			"Flow Element and outside any parallel group", err)
	}
}
