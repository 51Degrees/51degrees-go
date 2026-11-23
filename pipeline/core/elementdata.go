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
	"sync"
	"sync/atomic"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core/typedmap"
)

// ElementData is a container, within Flow Data, for the Property values relating
// to a particular Flow Element. Values are set by the Element during processing
// and retrieved using the string key of the Property.
//
// An instance is written only by the Flow Element that provides it, or by
// the Flow Elements that share its key, and only while they process. A write
// once they have finished panics with ErrElementDataFinished. An instance is
// not safe for writing from several goroutines at once, nor for reading
// while it is being written. Once its Flow Element has finished, the
// instance may be read from several goroutines at once, as the members of a
// parallel group read it, so an implementation must allow that.
//
// The exception is Element Data that reads its values from native results,
// such as the Device Detection DeviceData and the IP Intelligence Data. Such
// Element Data takes no lock and is not safe to use from several goroutines
// at once. Code that reads one instance from several goroutines, as the
// members of a parallel group after the Engine that adds it may, is
// responsible for thread-safe access, for example with a sync.Mutex the
// readers share. The Flow Elements of this module take no such lock when
// they read other Element Data, so a shared lock protects those reads only
// in a parallel group where none of them reads that instance.
//
// Keeping a reference to Element Data after the Flow Data that holds it is
// closed is not supported, because a pool may give the instance to another
// request, and nothing checks for it.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/conceptual-overview.md#element-data
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/thread-safety.md#element-data
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L196-L221
type ElementData interface {
	// Get returns the value of a Property by its (case-insensitive) string name.
	// It returns an error if the Property is not present.
	Get(propertyName string) (any, error)
	// Set stores the value of a Property. It panics with
	// ErrElementDataFinished once the Flow Elements that write the instance
	// have finished processing.
	Set(propertyName string, value any)
	// AsMap returns all Property values keyed by normalized (lower-case) name.
	AsMap() map[string]any
}

// FlowDataAdded is implemented by Element Data that needs to know where it
// is held, such as Aspect Data, which explains a missing Property from the
// Engine that populates it. A Flow Data calls it when it stores the Element
// Data.
type FlowDataAdded interface {
	// AddedToFlowData is given the Flow Data, the key the Element Data is
	// stored under, and the Element Data as stored, which is the type that
	// embeds the implementation.
	AddedToFlowData(fd FlowData, key string, data ElementData)
}

// WriteGuard records whether the Flow Elements that write an Element Data
// have finished processing. The Pipeline marks it when they finish, and opens
// it again when a Flow Element sharing the key starts. Element Data that does
// not embed BaseElementData embeds a WriteGuard and calls CheckWrite at the
// start of each method that writes a value.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/thread-safety.md?plain=1#L141-L149
type WriteGuard struct {
	finished atomic.Bool
}

// CheckWrite panics with ErrElementDataFinished, naming what was written,
// once the Flow Elements that write the Element Data have finished.
func (g *WriteGuard) CheckWrite(name string) {
	if g.finished.Load() {
		panic(fmt.Errorf("cannot set '%s': %w", name, ErrElementDataFinished))
	}
}

func (g *WriteGuard) finishWriting()  { g.finished.Store(true) }
func (g *WriteGuard) openForWriting() { g.finished.Store(false) }

// writeGuarded is Element Data that holds a WriteGuard.
type writeGuarded interface {
	finishWriting()
	openForWriting()
}

// guardOf gives the WriteGuard of d, looking through views that wrap other
// Element Data, or nil.
func guardOf(d ElementData) writeGuarded {
	for d != nil {
		if g, ok := d.(writeGuarded); ok {
			return g
		}
		u, ok := d.(ElementDataUnwrapper)
		if !ok {
			return nil
		}
		d = u.UnwrapElementData()
	}
	return nil
}

// BaseElementData is the default ElementData implementation that concrete
// Element Data types embed. It stores Property values in a typed map.
type BaseElementData struct {
	WriteGuard
	values typedmap.Map
	// shared is set when something beyond the Flow Data holds the instance,
	// as a cache does, so it never goes back to the pool.
	shared atomic.Bool
}

// baseElementDataPool holds BaseElementData for reuse. NewBaseElementData
// takes from it, and a Flow Data's Close gives back what it holds.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/resource-cleanup.md?plain=1#L87-L101
var baseElementDataPool = sync.Pool{New: func() any {
	return &BaseElementData{values: typedmap.NewMap()}
}}

// NewBaseElementData gives a BaseElementData with no values, open for
// writing, from a pool. Its values are held in a map that is not safe for
// writing from several goroutines at once, because only the Flow Element that
// creates the instance writes it. Once that Flow Element has finished,
// several goroutines can read the instance at once.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L143-L164
func NewBaseElementData() *BaseElementData {
	return baseElementDataPool.Get().(*BaseElementData)
}

// pooled is Element Data that goes back to a pool when the Flow Data that
// holds it is closed.
type pooled interface{ release() }

// release empties d and gives it back to the pool, unless it is shared.
func (d *BaseElementData) release() {
	if d.shared.Load() || d.values == nil {
		return
	}
	typedmap.Clear(d.values)
	d.openForWriting()
	baseElementDataPool.Put(d)
}

// sharable is Element Data that can be marked as shared.
type sharable interface{ markShared() }

func (d *BaseElementData) markShared() { d.shared.Store(true) }

// sharedFlag is Element Data that records whether it is shared.
type sharedFlag interface{ isShared() bool }

func (d *BaseElementData) isShared() bool { return d.shared.Load() }

// isShared reports whether d is marked shared, as Element Data a cache holds
// is.
func isShared(d ElementData) bool {
	s, ok := d.(sharedFlag)
	return ok && s.isShared()
}

// MarkShared records that d is held beyond the Flow Data that holds it, as a
// cache holds Element Data for other requests, so it never goes back to a
// pool when that Flow Data is closed.
func MarkShared(d ElementData) {
	for d != nil {
		if s, ok := d.(sharable); ok {
			s.markShared()
			return
		}
		u, ok := d.(ElementDataUnwrapper)
		if !ok {
			return
		}
		d = u.UnwrapElementData()
	}
}

// Get returns the value of a Property by name, or an error if absent.
func (d *BaseElementData) Get(propertyName string) (any, error) {
	v, ok := d.values.GetString(propertyName)
	if !ok {
		return nil, NewDataError("property '" + propertyName + "' is not present in this element data")
	}
	return v, nil
}

// Set stores a Property value. It panics with ErrElementDataFinished once the
// Flow Elements that write the instance have finished processing.
func (d *BaseElementData) Set(propertyName string, value any) {
	d.CheckWrite(propertyName)
	d.values.AddString(propertyName, value)
}

// AsMap returns a copy of all Property values.
func (d *BaseElementData) AsMap() map[string]any { return d.values.AsMap() }

// GetPropertyAs retrieves a Property value from Element Data and converts it to
// T. It is the strongly-typed accessor analogue described in access-to-results.
func GetPropertyAs[T any](d ElementData, propertyName string) (T, error) {
	var zero T
	raw, err := d.Get(propertyName)
	if err != nil {
		return zero, err
	}
	v, ok := raw.(T)
	if !ok {
		return zero, NewDataError("property '" + propertyName + "' is not of the requested type")
	}
	return v, nil
}
