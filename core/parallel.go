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
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// ParallelElements is a Flow Element that contains a set of child elements and
// executes them concurrently when processed. It is the mechanism behind the
// optional parallel-processing feature and the AddParallel builder method.
//
// A Pipeline that contains ParallelElements MUST be configured as concurrent so
// that Flow Data uses thread-safe storage. Each child element MUST write to a
// distinct Element Data key.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/advanced-features/parallel-processing.md
type ParallelElements struct {
	*BaseElement
	children []FlowElement
}

// NewParallelElements groups children to be executed in parallel.
func NewParallelElements(logger *slog.Logger, children ...FlowElement) *ParallelElements {
	keys := make([]string, len(children))
	var props []PropertyMetadata
	filters := make([]EvidenceKeyFilter, 0, len(children))
	for i, c := range children {
		keys[i] = c.DataKey()
		props = append(props, c.Properties()...)
		filters = append(filters, c.EvidenceKeyFilter())
	}
	base := NewBaseElement("parallel["+strings.Join(keys, "+")+"]", NewAggregateFilter(filters...), logger)
	base.SetProperties(props)
	return &ParallelElements{BaseElement: base, children: children}
}

// Children returns the contained elements.
func (pe *ParallelElements) Children() []FlowElement {
	out := make([]FlowElement, len(pe.children))
	copy(out, pe.children)
	return out
}

// Process runs all child elements concurrently and waits for them to complete.
// Errors and panics from children are recorded against the Flow Data rather
// than aborting the others.
func (pe *ParallelElements) Process(data FlowData) error {
	var wg sync.WaitGroup
	for _, c := range pe.children {
		wg.Add(1)
		go func(c FlowElement) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					data.AddError(fmt.Errorf("panic processing element '%s': %v", c.DataKey(), r), c)
				}
			}()
			if err := c.Process(data); err != nil {
				data.AddError(err, c)
			}
		}(c)
	}
	wg.Wait()
	return nil
}

// OnPipelineAdded notifies the ParallelElements and all its children.
func (pe *ParallelElements) OnPipelineAdded(p Pipeline) error {
	if err := pe.BaseElement.OnPipelineAdded(p); err != nil {
		return err
	}
	for _, c := range pe.children {
		if err := c.OnPipelineAdded(p); err != nil {
			return err
		}
	}
	return nil
}

// Close closes all child elements.
func (pe *ParallelElements) Close() error {
	var firstErr error
	for _, c := range pe.children {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// AddParallel adds a group of elements to be executed in parallel, after any
// previously added elements complete.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/pipeline-configuration.md#pipelines
func (b *PipelineBuilder) AddParallel(elements ...FlowElement) *PipelineBuilder {
	logger := b.logger
	if logger == nil {
		logger = slog.Default()
	}
	return b.AddFlowElement(NewParallelElements(logger, elements...))
}
