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
	"sync"
)

// Pipeline is an immutable container that groups multiple Flow Elements into a
// single process. Elements are executed sequentially in the order they were
// added (unless a ParallelElements grouping is used). A Pipeline can create many
// Flow Data instances and is expected to be used as a singleton.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/conceptual-overview.md#pipeline
type Pipeline interface {
	// CreateFlowData creates a new Flow Data bound to this Pipeline.
	CreateFlowData() FlowData
	// FlowElements returns the elements in flow order (a copy).
	FlowElements() []FlowElement
	// GetElement returns the element with the given data key, or nil.
	GetElement(dataKey string) FlowElement
	// Properties returns the aggregated Property metadata for all elements.
	Properties() []PropertyMetadata
	// EvidenceKeyFilter returns a filter that includes the Evidence keys of every
	// element in the Pipeline.
	EvidenceKeyFilter() EvidenceKeyFilter
	// IsConcurrent reports whether Flow Data created by this Pipeline uses
	// thread-safe storage (required for parallel processing).
	IsConcurrent() bool
	// IsSuppressProcessExceptions reports whether processing errors are logged
	// rather than returned. Exposed so wrappers such as the web integration can
	// make the same decision.
	// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/exception-handling.md
	IsSuppressProcessExceptions() bool
	// Close releases the resources held by the Pipeline's elements.
	Close() error
}

type pipeline struct {
	elements                  []FlowElement
	logger                    *slog.Logger
	concurrent                bool
	suppressProcessExceptions bool
	properties                []PropertyMetadata
	filter                    EvidenceKeyFilter

	closeOnce sync.Once
	closeErr  error
	closed    bool
	mu        sync.Mutex
	inFlight  sync.WaitGroup
}

func (p *pipeline) CreateFlowData() FlowData { return newFlowData(p) }

// isClosed reports whether the Pipeline has been closed.
func (p *pipeline) isClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

func (p *pipeline) FlowElements() []FlowElement {
	out := make([]FlowElement, len(p.elements))
	copy(out, p.elements)
	return out
}

func (p *pipeline) GetElement(dataKey string) FlowElement {
	return findElement(p.elements, dataKey)
}

// childLister is implemented by container elements (e.g. ParallelElements) so
// their children can be located by key.
type childLister interface {
	Children() []FlowElement
}

// findElement locates an element by data key, recursing into parallel groups so
// a child element can be found by key even though it is wrapped.
func findElement(elements []FlowElement, dataKey string) FlowElement {
	for _, e := range elements {
		if lower(e.DataKey()) == lower(dataKey) {
			return e
		}
		if group, ok := e.(childLister); ok {
			if child := findElement(group.Children(), dataKey); child != nil {
				return child
			}
		}
	}
	return nil
}

func (p *pipeline) Properties() []PropertyMetadata {
	out := make([]PropertyMetadata, len(p.properties))
	copy(out, p.properties)
	return out
}

func (p *pipeline) EvidenceKeyFilter() EvidenceKeyFilter { return p.filter }
func (p *pipeline) IsConcurrent() bool                   { return p.concurrent }
func (p *pipeline) IsSuppressProcessExceptions() bool    { return p.suppressProcessExceptions }

// process runs every element over the Flow Data, collecting errors, and decides
// whether to return them or log them based on the suppress setting. All elements
// are run even if an earlier element fails, so that the caller receives a
// complete picture of what went wrong.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/exception-handling.md#process-function
func (p *pipeline) process(fd *flowData) error {
	// Register as in-flight under the lock and before the closed check so that
	// Close, which sets closed under the same lock and then waits, cannot tear
	// down an element while this call is still using it.
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrPipelineClosed
	}
	p.inFlight.Add(1)
	p.mu.Unlock()
	defer p.inFlight.Done()

	for _, e := range p.elements {
		p.processElement(e, fd)
	}

	if !fd.HasErrors() {
		return nil
	}
	if p.suppressProcessExceptions {
		for _, fe := range fd.Errors() {
			key := ""
			if fe.Element != nil {
				key = fe.Element.DataKey()
			}
			p.logger.Error("error during flow element processing", "element", key, "error", fe.Err)
		}
		return nil
	}
	return &ProcessError{Errors: fd.Errors()}
}

// processElement runs a single element, converting any panic into a collected
// error so that one misbehaving element cannot crash the Pipeline.
func (p *pipeline) processElement(e FlowElement, fd *flowData) {
	defer func() {
		if r := recover(); r != nil {
			fd.AddError(fmt.Errorf("panic processing element '%s': %v", e.DataKey(), r), e)
		}
	}()
	if err := e.Process(fd); err != nil {
		fd.AddError(err, e)
	}
}

func (p *pipeline) Close() error {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()
		// Wait for in-flight processing to finish so a stateful element is not
		// closed underneath a running Process.
		p.inFlight.Wait()
		for _, e := range p.elements {
			if err := e.Close(); err != nil && p.closeErr == nil {
				p.closeErr = err
			}
		}
	})
	return p.closeErr
}
