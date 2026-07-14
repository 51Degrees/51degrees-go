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
	"sync"
)

// FlowElement is a black box that takes a Flow Data, reads Evidence and/or
// earlier Element Data, and MAY add an instance of its own Element Data.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/conceptual-overview.md#flow-element
//
// Concrete elements embed BaseElement (which supplies default implementations
// of every method except Process) and implement Process themselves.
type FlowElement interface {
	// DataKey is the string key identifying this element's Element Data within
	// Flow Data.
	DataKey() string
	// Process reads from and writes to the supplied Flow Data.
	Process(data FlowData) error
	// EvidenceKeyFilter describes the Evidence keys this element makes use of.
	EvidenceKeyFilter() EvidenceKeyFilter
	// Properties returns metadata describing the Properties this element can
	// populate.
	Properties() []PropertyMetadata
	// OnPipelineAdded is called when the element is added to a Pipeline. It
	// allows the element to track the Pipelines it belongs to (multi-pipeline
	// support) and to fail if it does not support being added more than once.
	OnPipelineAdded(p Pipeline) error
	// IsConcurrent reports whether the element processes Flow Data on multiple
	// goroutines (for example ParallelElements). A Pipeline containing any such
	// element uses thread-safe Flow Data storage. This mirrors the .NET
	// reference, where IsConcurrent propagates up from the elements.
	IsConcurrent() bool
	// Close releases any resources held by the element.
	// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/resource-cleanup.md
	Close() error
}

// BaseElement supplies the boilerplate behavior shared by all Flow Elements:
// the data key, evidence filter, property metadata, logger, and Pipeline
// tracking. Concrete elements embed it by pointer and implement Process.
type BaseElement struct {
	dataKey    string
	filter     EvidenceKeyFilter
	properties []PropertyMetadata
	logger     *slog.Logger

	mu        sync.Mutex
	pipelines []Pipeline
}

// NewBaseElement creates a BaseElement with the given data key and evidence
// filter. If logger is nil, the default slog logger is used. Property metadata
// is set separately via SetProperties so it can reference the concrete element.
func NewBaseElement(dataKey string, filter EvidenceKeyFilter, logger *slog.Logger) *BaseElement {
	if logger == nil {
		logger = slog.Default()
	}
	if filter == nil {
		filter = NewWhitelistFilter()
	}
	return &BaseElement{dataKey: dataKey, filter: filter, logger: logger}
}

// SetProperties stores the element's Property metadata. Concrete elements call
// this after construction, with PropertyMetadata.Element pointing at themselves.
func (b *BaseElement) SetProperties(props []PropertyMetadata) { b.properties = props }

// SetEvidenceKeyFilter replaces the element's evidence filter.
func (b *BaseElement) SetEvidenceKeyFilter(f EvidenceKeyFilter) { b.filter = f }

// DataKey returns the element's data key.
func (b *BaseElement) DataKey() string { return b.dataKey }

// IsConcurrent reports whether the element processes on multiple goroutines.
// The default is false; ParallelElements overrides it.
func (b *BaseElement) IsConcurrent() bool { return false }

// EvidenceKeyFilter returns the element's evidence filter.
func (b *BaseElement) EvidenceKeyFilter() EvidenceKeyFilter { return b.filter }

// Properties returns the element's Property metadata.
func (b *BaseElement) Properties() []PropertyMetadata { return b.properties }

// Logger returns the element's logger.
func (b *BaseElement) Logger() *slog.Logger { return b.logger }

// OnPipelineAdded records the Pipeline the element has been added to. The
// default implementation supports being added to multiple Pipelines.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/advanced-features/multi-pipeline-elements.md
func (b *BaseElement) OnPipelineAdded(p Pipeline) error {
	b.mu.Lock()
	b.pipelines = append(b.pipelines, p)
	b.mu.Unlock()
	return nil
}

// removePipeline undoes an OnPipelineAdded. Build uses it to roll back
// membership recorded on earlier elements when a later element rejects the
// pipeline, so a shared element does not retain a reference to a pipeline that
// is discarded and never closed.
func (b *BaseElement) removePipeline(p Pipeline) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, pp := range b.pipelines {
		if pp == p {
			b.pipelines = append(b.pipelines[:i], b.pipelines[i+1:]...)
			return
		}
	}
}

// Pipelines returns the Pipelines this element has been added to.
func (b *BaseElement) Pipelines() []Pipeline {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Pipeline, len(b.pipelines))
	copy(out, b.pipelines)
	return out
}

// Close releases resources. The default implementation does nothing.
func (b *BaseElement) Close() error { return nil }
