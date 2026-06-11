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
}

func (p *pipeline) CreateFlowData() FlowData { return newFlowData(p) }

func (p *pipeline) FlowElements() []FlowElement {
	out := make([]FlowElement, len(p.elements))
	copy(out, p.elements)
	return out
}

func (p *pipeline) GetElement(dataKey string) FlowElement {
	for _, e := range p.elements {
		if lower(e.DataKey()) == lower(dataKey) {
			return e
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
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return ErrPipelineClosed
	}

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
		for _, e := range p.elements {
			if err := e.Close(); err != nil && p.closeErr == nil {
				p.closeErr = err
			}
		}
	})
	return p.closeErr
}
