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
	"sync"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core/typedmap"
)

// FlowData is the fundamental unit of work within the Pipeline API. It carries
// the input Evidence into the Pipeline and the output Element Data back to the
// caller. A Flow Data belongs to exactly one Pipeline and can be processed only
// once.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/conceptual-overview.md#flow-data
type FlowData interface {
	// AddEvidence adds a single Evidence key-value pair, replacing any value held
	// under the same key when the case is ignored. The caller adds Evidence
	// before processing, and a Flow Element may add Evidence while it processes,
	// for the Flow Elements after it and the caller to read. A call while a
	// parallel group runs, or after processing has finished, is ignored and
	// logged. Returns the Flow Data for chaining.
	// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/evidence.md#adding-evidence-values
	AddEvidence(key string, value any) FlowData
	// AddEvidenceMap adds multiple Evidence values.
	AddEvidenceMap(values map[string]any) FlowData
	// Evidence returns the (read-only) Evidence collection.
	Evidence() Evidence
	// Process runs the owning Pipeline's elements over this Flow Data. It may only
	// be called once.
	Process() error
	// Pipeline returns the Pipeline that created this Flow Data.
	Pipeline() Pipeline

	// Get returns the Element Data stored under the given element data key.
	Get(elementDataKey string) (ElementData, error)
	// GetOrAdd returns the Element Data for key, creating it with ctor if absent.
	// Flow Elements use this to obtain their own Element Data during Process.
	GetOrAdd(key string, ctor func(FlowData) ElementData) ElementData
	// ElementDataAsMap returns all Element Data keyed by element data key.
	ElementDataAsMap() map[string]ElementData

	// Errors returns the errors collected during processing.
	Errors() []FlowError
	// HasErrors reports whether any errors were collected during processing.
	HasErrors() bool
	// AddError records an error against the element that produced it.
	AddError(err error, element FlowElement)

	// markStarted is called internally to enforce single processing. It returns
	// false if processing has already started.
	markStarted() bool
	// isStarted reports whether processing has started.
	isStarted() bool
}

// flowData is the default FlowData implementation.
type flowData struct {
	pipeline *pipeline
	evidence *evidence
	data     typedmap.Map

	mu       sync.Mutex
	errs     []FlowError
	started  bool
	finished bool
	// parallel counts the parallel groups running, which take no Evidence.
	parallel int

	goaMu sync.Mutex // serializes GetOrAdd so it is atomic under parallel processing
}

func newFlowData(p *pipeline) *flowData {
	var data typedmap.Map
	if p.concurrent {
		data = typedmap.NewConcurrentMap()
	} else {
		data = typedmap.NewMap()
	}
	return &flowData{pipeline: p, evidence: newEvidence(), data: data}
}

func (fd *flowData) AddEvidence(key string, value any) FlowData {
	// Hold the lock across the state checks and the write so a group starting
	// or processing finishing cannot slip between them.
	fd.mu.Lock()
	defer fd.mu.Unlock()
	if fd.finished {
		fd.pipeline.logger.Warn("evidence added after processing finished; ignoring", "key", key)
		return fd
	}
	if fd.parallel > 0 {
		fd.pipeline.logger.Warn("evidence added while a parallel group is running; ignoring", "key", key)
		return fd
	}
	fd.evidence.put(key, value)
	return fd
}

func (fd *flowData) AddEvidenceMap(values map[string]any) FlowData {
	for k, v := range values {
		fd.AddEvidence(k, v)
	}
	return fd
}

func (fd *flowData) Evidence() Evidence { return fd.evidence }

func (fd *flowData) Process() error {
	// Reject a closed Pipeline before marking the Flow Data started, so a
	// transient closed-pipeline error does not permanently poison it.
	if fd.pipeline.isClosed() {
		return ErrPipelineClosed
	}
	if !fd.markStarted() {
		return ErrAlreadyProcessed
	}
	defer fd.markFinished()
	return fd.pipeline.process(fd)
}

func (fd *flowData) Pipeline() Pipeline { return fd.pipeline }

func (fd *flowData) Get(elementDataKey string) (ElementData, error) {
	if !fd.isStarted() {
		return nil, ErrNotProcessed
	}
	raw, ok := fd.data.GetString(elementDataKey)
	if !ok {
		return nil, NewDataError("no element data found for key '" + elementDataKey + "'")
	}
	return raw.(ElementData), nil
}

func (fd *flowData) GetOrAdd(key string, ctor func(FlowData) ElementData) ElementData {
	// Serialize so the check-then-add is atomic; this matters when elements run
	// in parallel (e.g. two Translation Engines sharing one Element Data).
	fd.goaMu.Lock()
	defer fd.goaMu.Unlock()
	if raw, ok := fd.data.GetString(key); ok {
		return raw.(ElementData)
	}
	d := ctor(fd)
	fd.data.AddString(key, d)
	return d
}

func (fd *flowData) ElementDataAsMap() map[string]ElementData {
	keys := fd.data.Keys()
	out := make(map[string]ElementData, len(keys))
	for _, k := range keys {
		if raw, ok := fd.data.GetString(k); ok {
			out[k] = raw.(ElementData)
		}
	}
	return out
}

func (fd *flowData) Errors() []FlowError {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	out := make([]FlowError, len(fd.errs))
	copy(out, fd.errs)
	return out
}

func (fd *flowData) HasErrors() bool {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	return len(fd.errs) > 0
}

func (fd *flowData) AddError(err error, element FlowElement) {
	fd.mu.Lock()
	fd.errs = append(fd.errs, FlowError{Element: element, Err: err})
	fd.mu.Unlock()
}

func (fd *flowData) markStarted() bool {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	if fd.started {
		return false
	}
	fd.started = true
	return true
}

// markFinished records that processing is complete, after which Evidence takes
// no more values.
func (fd *flowData) markFinished() {
	fd.mu.Lock()
	fd.finished = true
	fd.mu.Unlock()
}

// enterParallel and leaveParallel bracket a parallel group, during which
// Evidence takes no values.
func (fd *flowData) enterParallel() {
	fd.mu.Lock()
	fd.parallel++
	fd.mu.Unlock()
}

func (fd *flowData) leaveParallel() {
	fd.mu.Lock()
	fd.parallel--
	fd.mu.Unlock()
}

func (fd *flowData) isStarted() bool {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	return fd.started
}

// ElementDataUnwrapper is implemented by per-Flow-Data Element Data views that
// wrap another instance (for example a cache-hit view). Type-based access
// unwraps it so the underlying concrete Element Data can still be matched.
type ElementDataUnwrapper interface {
	// UnwrapElementData returns the wrapped Element Data.
	UnwrapElementData() ElementData
}

// asType converts d to T, unwrapping an ElementDataUnwrapper if necessary.
func asType[T ElementData](d ElementData) (T, bool) {
	if v, ok := d.(T); ok {
		return v, true
	}
	if u, ok := d.(ElementDataUnwrapper); ok {
		if v, ok := u.UnwrapElementData().(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// Get retrieves the Element Data of type T from the Flow Data by matching its
// type. It returns an error when no Element Data matches, and also when more
// than one matches (the ambiguity is a configuration error rather than a
// silently non-deterministic choice, matching the reference implementation).
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/access-to-results.md
func Get[T ElementData](fd FlowData) (T, error) {
	var zero T
	if !fd.isStarted() {
		return zero, ErrNotProcessed
	}
	var found T
	matches := 0
	for _, d := range fd.ElementDataAsMap() {
		if v, ok := asType[T](d); ok {
			found = v
			matches++
		}
	}
	switch matches {
	case 0:
		return zero, NewDataError("no element data of the requested type was found")
	case 1:
		return found, nil
	default:
		return zero, NewDataError("more than one element data of the requested type was found; retrieve it by key instead")
	}
}

// GetFromElement retrieves the Element Data produced by the given element and
// converts it to T.
func GetFromElement[T ElementData](fd FlowData, element FlowElement) (T, error) {
	var zero T
	d, err := fd.Get(element.DataKey())
	if err != nil {
		return zero, err
	}
	v, ok := asType[T](d)
	if !ok {
		return zero, NewDataError("element data for '" + element.DataKey() + "' is not of the requested type")
	}
	return v, nil
}

// GetEvidenceOrProperty implements the suggested helper for "adding" Evidence:
// it returns the value of a Property (matched by field name) from Element Data
// if exactly one element produced it, otherwise it falls back to the Evidence
// value for the full key. An error is returned if multiple element data
// instances contain a Property with the requested field name.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/evidence.md#adding-evidence-values
func GetEvidenceOrProperty(fd FlowData, key string) (any, bool, error) {
	_, field := SplitEvidenceKey(key)
	var found any
	matches := 0
	for _, d := range fd.ElementDataAsMap() {
		if v, err := d.Get(field); err == nil {
			found = v
			matches++
		}
	}
	if matches > 1 {
		return nil, false, NewDataError("multiple element data instances contain property '" + field + "'")
	}
	if matches == 1 {
		return found, true, nil
	}
	v, ok := fd.Evidence().Get(key)
	return v, ok, nil
}
