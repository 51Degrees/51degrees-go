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

import "log/slog"

// PipelineBuilder constructs immutable Pipeline instances. Flow Elements are
// added in flow order. This is the Go analogue of the fluent Pipeline builder
// used in the reference implementations.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/conceptual-overview.md#pipeline-builder
type PipelineBuilder struct {
	elements                  []FlowElement
	logger                    *slog.Logger
	concurrent                bool
	suppressProcessExceptions bool
}

// NewPipelineBuilder creates an empty PipelineBuilder.
func NewPipelineBuilder() *PipelineBuilder { return &PipelineBuilder{} }

// AddFlowElement appends an element to the Pipeline. Elements are executed in
// the order they are added.
func (b *PipelineBuilder) AddFlowElement(e FlowElement) *PipelineBuilder {
	b.elements = append(b.elements, e)
	return b
}

// SetLogger sets the logger used by the Pipeline. If unset, slog.Default is used.
func (b *PipelineBuilder) SetLogger(l *slog.Logger) *PipelineBuilder {
	b.logger = l
	return b
}

// SetConcurrent configures whether Flow Data created by the Pipeline uses
// thread-safe storage. This must be enabled when using parallel processing.
func (b *PipelineBuilder) SetConcurrent(v bool) *PipelineBuilder {
	b.concurrent = v
	return b
}

// SetSuppressProcessExceptions configures whether processing errors are logged
// rather than returned from Process.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/exception-handling.md
func (b *PipelineBuilder) SetSuppressProcessExceptions(v bool) *PipelineBuilder {
	b.suppressProcessExceptions = v
	return b
}

// Build creates the immutable Pipeline. It aggregates Property metadata and the
// Evidence key filter across all elements, then notifies each element that it
// has been added to the Pipeline.
func (b *PipelineBuilder) Build() (Pipeline, error) {
	logger := b.logger
	if logger == nil {
		logger = slog.Default()
	}

	elements := make([]FlowElement, len(b.elements))
	copy(elements, b.elements)

	var properties []PropertyMetadata
	filters := make([]EvidenceKeyFilter, 0, len(elements))
	for _, e := range elements {
		properties = append(properties, e.Properties()...)
		filters = append(filters, e.EvidenceKeyFilter())
	}

	p := &pipeline{
		elements:                  elements,
		logger:                    logger,
		concurrent:                b.concurrent,
		suppressProcessExceptions: b.suppressProcessExceptions,
		properties:                properties,
		filter:                    NewAggregateFilter(filters...),
	}

	for _, e := range elements {
		if err := e.OnPipelineAdded(p); err != nil {
			return nil, NewConfigurationError("element '"+e.DataKey()+"' could not be added to the pipeline", err)
		}
	}
	return p, nil
}
