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
	"maps"
	"sync"
	"sync/atomic"
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
	// for the Flow Elements after it and the caller to read. A member of a
	// parallel group adds to its own view of the Flow Data, so what it adds is
	// dropped when the group finishes, as FlowElement.Process says. A call
	// after processing has finished panics with ErrFlowDataFinished. The
	// caller must not add Evidence while processing runs, because such a call
	// cannot be told apart from one made by a Flow Element.
	// Returns the Flow Data for chaining.
	// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/evidence.md#adding-evidence-values
	AddEvidence(key string, value any) FlowData
	// AddEvidenceMap adds multiple Evidence values, as AddEvidence does.
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
	// Adding Element Data after processing has finished panics with
	// ErrFlowDataFinished.
	GetOrAdd(key string, ctor func(FlowData) ElementData) ElementData
	// ElementDataAsMap returns all Element Data keyed by element data key.
	ElementDataAsMap() map[string]ElementData

	// Errors returns the errors collected during processing.
	Errors() []FlowError
	// HasErrors reports whether any errors were collected during processing.
	HasErrors() bool
	// AddError records an error against the element that produced it. A call
	// after processing has finished panics with ErrFlowDataFinished.
	AddError(err error, element FlowElement)

	// Close gives the Element Data back to the pools it came from, apart from
	// any that a cache holds, and leaves the Flow Data holding none. Call it
	// once the Flow Data and its Element Data are no longer needed. Keeping a
	// reference to the Element Data after Close is not supported, because a
	// pool may give the instance to another request, and nothing checks for
	// it. A closed Flow Data cannot be processed, and a second Close does
	// nothing. It returns nil.
	// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/resource-cleanup.md?plain=1#L87-L101
	Close() error

	// markStarted is called internally to enforce single processing. It returns
	// false if processing has already started.
	markStarted() bool
	// isStarted reports whether processing has started.
	isStarted() bool
}

// flowData is the default FlowData implementation. It takes no lock, apart
// from the one the members of a parallel group add their errors under.
//
// Element Data is held in a list, newest first. A node never changes once it
// is in the list, so readers follow it with no lock. A Pipeline with a
// parallel group puts each new node in place with one compare and swap, so
// members can add at the same time, and any other Pipeline with a plain
// store.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/thread-safety.md?plain=1#L90-L118
type flowData struct {
	pipeline *pipeline
	evidence *evidence
	head     atomic.Pointer[elementNode]

	// errs is appended to with no lock, except by the members of a parallel
	// group, which hold errMu.
	errMu sync.Mutex
	errs  []FlowError

	started  atomic.Bool
	finished atomic.Bool
	closed   bool
	// parallel counts the parallel groups running, whose members add their
	// errors under errMu.
	parallel atomic.Int32
	// supplied is the Evidence as it was when processing started.
	supplied map[string]any
}

// elementNode holds one Element Data in a Flow Data's list.
type elementNode struct {
	key  string
	data ElementData
	next *elementNode
}

// find gives the node for key, already in lower case, from n onwards.
func (n *elementNode) find(key string) *elementNode {
	for ; n != nil; n = n.next {
		if n.key == key {
			return n
		}
	}
	return nil
}

func newFlowData(p *pipeline) *flowData {
	return &flowData{pipeline: p, evidence: newEvidence()}
}

// finishedPanic panics with ErrFlowDataFinished, saying what was added. It is
// called once processing has finished.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/thread-safety.md?plain=1#L115-L118
func finishedPanic(what string) {
	panic(fmt.Errorf("cannot add %s: %w", what, ErrFlowDataFinished))
}

func (fd *flowData) AddEvidence(key string, value any) FlowData {
	if fd.finished.Load() {
		finishedPanic("Evidence '" + key + "'")
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
	fd.supplied = fd.evidence.AsMap()
	defer fd.markFinished()
	return fd.pipeline.process(fd)
}

// SuppliedEvidence returns the Evidence as it was when processing of the Flow
// Data started, without the values that Flow Elements added while processing.
// Before processing starts it returns the Evidence held now. Each call returns
// a new map. It is for work that depends on the request alone, such as a cache
// validator.
func SuppliedEvidence(fd FlowData) map[string]any {
	if f := baseFlowData(fd); f != nil && f.supplied != nil {
		return maps.Clone(f.supplied)
	}
	return fd.Evidence().AsMap()
}

func (fd *flowData) Pipeline() Pipeline { return fd.pipeline }

func (fd *flowData) Get(elementDataKey string) (ElementData, error) {
	if !fd.isStarted() {
		return nil, ErrNotProcessed
	}
	n := fd.head.Load().find(lower(elementDataKey))
	if n == nil {
		return nil, NewDataError("no element data found for key '" + elementDataKey + "'")
	}
	return n.data, nil
}

// GetOrAdd gives the Element Data held under key, or adds the one ctor makes.
// In a Pipeline with a parallel group the new node goes in with a compare
// and swap, retried when another member added first.
func (fd *flowData) GetOrAdd(key string, ctor func(FlowData) ElementData) ElementData {
	k := lower(key)
	head := fd.head.Load()
	if n := head.find(k); n != nil {
		return n.data
	}
	if fd.finished.Load() {
		finishedPanic("Element Data '" + key + "'")
	}
	d := ctor(fd)
	if added, ok := d.(FlowDataAdded); ok {
		added.AddedToFlowData(fd, key, d)
	}
	n := &elementNode{key: k, data: d, next: head}
	if !fd.pipeline.concurrent {
		fd.head.Store(n)
		return d
	}
	for !fd.head.CompareAndSwap(head, n) {
		head = fd.head.Load()
		if m := head.find(k); m != nil {
			return m.data
		}
		n.next = head
	}
	return d
}

func (fd *flowData) ElementDataAsMap() map[string]ElementData {
	out := make(map[string]ElementData)
	for n := fd.head.Load(); n != nil; n = n.next {
		if _, ok := out[n.key]; !ok {
			out[n.key] = n.data
		}
	}
	return out
}

// Errors gives a copy of the errors. It takes the errors lock only while a
// parallel group runs.
func (fd *flowData) Errors() []FlowError {
	if fd.parallel.Load() > 0 {
		fd.errMu.Lock()
		defer fd.errMu.Unlock()
	}
	out := make([]FlowError, len(fd.errs))
	copy(out, fd.errs)
	return out
}

func (fd *flowData) HasErrors() bool {
	if fd.parallel.Load() > 0 {
		fd.errMu.Lock()
		defer fd.errMu.Unlock()
	}
	return len(fd.errs) > 0
}

// AddError records an error. The members of a parallel group add under the
// errors lock, and the error is added with no lock at any other time.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/thread-safety.md?plain=1#L108-L113
func (fd *flowData) AddError(err error, element FlowElement) {
	if fd.finished.Load() {
		finishedPanic(fmt.Sprintf("the error '%v'", err))
	}
	if fd.parallel.Load() > 0 {
		fd.errMu.Lock()
		defer fd.errMu.Unlock()
	}
	fd.errs = append(fd.errs, FlowError{Element: element, Err: err})
}

func (fd *flowData) markStarted() bool {
	return fd.started.CompareAndSwap(false, true)
}

// markFinished records that processing is complete. Every Element Data is
// then read only, and nothing more can be added.
func (fd *flowData) markFinished() {
	for n := fd.head.Load(); n != nil; n = n.next {
		if g := guardOf(n.data); g != nil {
			g.finishWriting()
		}
	}
	fd.finished.Store(true)
}

// Close gives each Element Data that came from a pool back to it. A view of
// a cached instance, and an instance marked shared, stay where they are.
func (fd *flowData) Close() error {
	if fd.closed {
		return nil
	}
	fd.closed = true
	fd.started.Store(true)
	fd.finished.Store(true)
	for n := fd.head.Load(); n != nil; n = n.next {
		if p, ok := n.data.(pooled); ok {
			p.release()
		}
	}
	fd.head.Store(nil)
	return nil
}

// openForWriting opens the Element Data held under keys, in lower case, for
// writing again, as a Flow Element that shares a key starts to process.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/thread-safety.md?plain=1#L143-L149
func (fd *flowData) openForWriting(keys []string) {
	head := fd.head.Load()
	for _, k := range keys {
		if n := head.find(k); n != nil {
			if g := guardOf(n.data); g != nil {
				g.openForWriting()
			}
		}
	}
}

// finishWriting records that a Flow Element has finished, which makes the
// Element Data under keys, and any added since the list began at since,
// read only.
func (fd *flowData) finishWriting(since *elementNode, keys []string) {
	head := fd.head.Load()
	for n := head; n != nil && n != since; n = n.next {
		if g := guardOf(n.data); g != nil {
			g.finishWriting()
		}
	}
	for _, k := range keys {
		if n := since.find(k); n != nil {
			if g := guardOf(n.data); g != nil {
				g.finishWriting()
			}
		}
	}
}

// enterParallel and leaveParallel bracket a parallel group, during which
// errors are added under the errors lock.
func (fd *flowData) enterParallel() {
	fd.parallel.Add(1)
}

func (fd *flowData) leaveParallel() {
	fd.parallel.Add(-1)
}

func (fd *flowData) isStarted() bool {
	return fd.started.Load()
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
