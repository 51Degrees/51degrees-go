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
	"sync/atomic"
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
	// Close stops the Pipeline and releases the resources held by its
	// elements, unless the builder's SetAutoCloseElements(false) left them for
	// the caller to close.
	Close() error
}

type pipeline struct {
	elements                  []FlowElement
	logger                    *slog.Logger
	concurrent                bool
	suppressProcessExceptions bool
	closeElements             bool
	filter                    EvidenceKeyFilter

	// properties is the elements' Property metadata, gathered on first use
	// and again until every element has loaded its Properties, after which
	// it is kept.
	properties       []PropertyMetadata
	propertiesMu     sync.Mutex
	propertiesLoaded atomic.Bool

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

// Properties returns the Property metadata of every element. An element
// whose Properties arrive after the Pipeline is built, such as a Cloud Aspect
// Engine after a transient failure, is asked again on each call until it has
// loaded them, so a consumer that reads the metadata then sees them.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#consequences-for-consumers-of-the-engine
func (p *pipeline) Properties() []PropertyMetadata {
	props := p.currentProperties()
	out := make([]PropertyMetadata, len(props))
	copy(out, props)
	return out
}

func (p *pipeline) currentProperties() []PropertyMetadata {
	if p.propertiesLoaded.Load() {
		return p.properties
	}
	p.propertiesMu.Lock()
	defer p.propertiesMu.Unlock()
	if !p.propertiesLoaded.Load() {
		var loaded bool
		p.properties, loaded = gatherProperties(p.elements)
		p.propertiesLoaded.Store(loaded)
	}
	return p.properties
}

// PropertiesLoader is implemented by a Flow Element whose Property metadata
// can arrive after it is built, such as a Cloud Aspect Engine whose
// Properties come from the cloud service. Until PropertiesLoaded is true, a
// Pipeline gathers the elements' Property metadata again each time it is
// asked for.
type PropertiesLoader interface {
	// PropertiesLoaded reports whether the element's Properties are known.
	PropertiesLoaded() bool
}

// gatherProperties returns the Property metadata of elements, and whether
// every element has loaded its Properties.
func gatherProperties(elements []FlowElement) ([]PropertyMetadata, bool) {
	var props []PropertyMetadata
	loaded := true
	for _, e := range elements {
		props = append(props, e.Properties()...)
		if l, ok := e.(PropertiesLoader); ok && !l.PropertiesLoaded() {
			loaded = false
		}
	}
	return props, loaded
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
		if !p.closeElements {
			return
		}
		for _, e := range p.elements {
			if err := e.Close(); err != nil && p.closeErr == nil {
				p.closeErr = err
			}
		}
	})
	return p.closeErr
}
