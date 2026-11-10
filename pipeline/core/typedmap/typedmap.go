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

// Package typedmap provides a string-keyed container that can store values of
// any type, with type-safe generic accessors layered on top.
//
// It is the Go analogue of the reference implementation's TypedKeyMap /
// ITypedKey<T> (see
// https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/access-to-results.md).
//
// Go interfaces cannot declare type-parameterized methods, so the typed
// accessors are provided as package-level generic functions (Get, Add,
// GetOrAdd, ...) that operate on the Map interface rather than as methods.
package typedmap

import (
	"fmt"
	"strings"
	"sync"
)

// TypedKey is a named key that also carries the static type of the value it
// addresses. It mirrors ITypedKey<T> from the reference implementations.
type TypedKey[T any] struct {
	name string
}

// NewTypedKey creates a TypedKey for the given name. Names are compared
// case-insensitively, matching the access-to-results specification.
func NewTypedKey[T any](name string) TypedKey[T] {
	return TypedKey[T]{name: name}
}

// Name returns the string name of the key.
func (k TypedKey[T]) Name() string { return k.name }

// Map is a string-keyed store of arbitrary values. All key comparisons are
// case-insensitive (keys are normalized to lower case). Implementations are
// obtained via NewMap (not safe for writing from several goroutines at once)
// or NewConcurrentMap (safe for concurrent use).
type Map interface {
	// AddString stores value under name, replacing any existing entry.
	AddString(name string, value any)
	// GetString returns the value stored under name and whether it existed.
	GetString(name string) (any, bool)
	// GetOrAddString returns the existing value for name, or atomically stores
	// and returns ctor()'s result if absent, so concurrent callers all observe
	// the same stored value (ctor may run more than once but only the first
	// stored result is kept and returned to every caller).
	GetOrAddString(name string, ctor func() any) any
	// Remove deletes the entry stored under name, if present.
	Remove(name string)
	// Contains reports whether an entry exists for name.
	Contains(name string) bool
	// Keys returns the (normalized, lower-case) names of all stored entries.
	// The order is unspecified.
	Keys() []string
	// AsMap returns a shallow copy of the contents keyed by normalized name.
	AsMap() map[string]any
}

func normalize(name string) string { return strings.ToLower(name) }

// simpleMap is a non-thread-safe Map backed by a plain map.
type simpleMap struct {
	data map[string]any
}

// NewMap returns a Map that is not safe for writing from several goroutines
// at once, nor for reading while it is written. Several goroutines can read
// it at once while none writes it. This suits per-request data such as
// Element Data, which only the Flow Element that creates it writes, and which
// several goroutines read at once only after that Flow Element has finished.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/thread-safety.md#element-data
func NewMap() Map {
	return &simpleMap{data: make(map[string]any)}
}

func (m *simpleMap) AddString(name string, value any) { m.data[normalize(name)] = value }
func (m *simpleMap) GetString(name string) (any, bool) {
	v, ok := m.data[normalize(name)]
	return v, ok
}
func (m *simpleMap) GetOrAddString(name string, ctor func() any) any {
	n := normalize(name)
	if v, ok := m.data[n]; ok {
		return v
	}
	v := ctor()
	m.data[n] = v
	return v
}
func (m *simpleMap) Remove(name string)        { delete(m.data, normalize(name)) }
func (m *simpleMap) Contains(name string) bool { _, ok := m.data[normalize(name)]; return ok }
func (m *simpleMap) Keys() []string {
	keys := make([]string, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys
}
func (m *simpleMap) AsMap() map[string]any {
	out := make(map[string]any, len(m.data))
	for k, v := range m.data {
		out[k] = v
	}
	return out
}

// concurrentMap is a Map safe for concurrent use via an RWMutex.
type concurrentMap struct {
	mu   sync.RWMutex
	data map[string]any
}

// NewConcurrentMap returns a Map that is safe for concurrent use. This is used
// where the Pipeline is configured for concurrent or parallel processing.
func NewConcurrentMap() Map {
	return &concurrentMap{data: make(map[string]any)}
}

func (m *concurrentMap) AddString(name string, value any) {
	m.mu.Lock()
	m.data[normalize(name)] = value
	m.mu.Unlock()
}
func (m *concurrentMap) GetString(name string) (any, bool) {
	m.mu.RLock()
	v, ok := m.data[normalize(name)]
	m.mu.RUnlock()
	return v, ok
}
func (m *concurrentMap) GetOrAddString(name string, ctor func() any) any {
	n := normalize(name)
	m.mu.RLock()
	v, ok := m.data[n]
	m.mu.RUnlock()
	if ok {
		return v
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// Re-check under the write lock: another goroutine may have stored a value
	// between the read lock above and acquiring the write lock.
	if v, ok := m.data[n]; ok {
		return v
	}
	v = ctor()
	m.data[n] = v
	return v
}
func (m *concurrentMap) Remove(name string) {
	m.mu.Lock()
	delete(m.data, normalize(name))
	m.mu.Unlock()
}
func (m *concurrentMap) Contains(name string) bool {
	m.mu.RLock()
	_, ok := m.data[normalize(name)]
	m.mu.RUnlock()
	return ok
}
func (m *concurrentMap) Keys() []string {
	m.mu.RLock()
	keys := make([]string, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	m.mu.RUnlock()
	return keys
}
func (m *concurrentMap) AsMap() map[string]any {
	m.mu.RLock()
	out := make(map[string]any, len(m.data))
	for k, v := range m.data {
		out[k] = v
	}
	m.mu.RUnlock()
	return out
}

// ErrWrongType is returned (wrapped) when a stored value cannot be converted to
// the requested type.
type ErrWrongType struct {
	// Name is the key whose value could not be converted.
	Name string
	// Expected is the name of the requested type.
	Expected string
	// Actual is the value that was stored under the key.
	Actual any
}

// Error returns a message describing the type mismatch.
func (e *ErrWrongType) Error() string {
	return fmt.Sprintf("value for key %q is of type %T, not %s", e.Name, e.Actual, e.Expected)
}

// ErrNotFound is returned (wrapped) when no value exists for a key.
type ErrNotFound struct {
	// Name is the key that was not found.
	Name string
}

// Error returns a message naming the key that was not found.
func (e *ErrNotFound) Error() string { return fmt.Sprintf("no value found for key %q", e.Name) }

// Add stores value under the typed key.
func Add[T any](m Map, key TypedKey[T], value T) { m.AddString(key.Name(), value) }

// Get retrieves the value for a typed key, converting it to T.
func Get[T any](m Map, key TypedKey[T]) (T, error) { return GetByName[T](m, key.Name()) }

// GetByName retrieves the value stored under name and converts it to T.
func GetByName[T any](m Map, name string) (T, error) {
	var zero T
	raw, ok := m.GetString(name)
	if !ok {
		return zero, &ErrNotFound{Name: name}
	}
	v, ok := raw.(T)
	if !ok {
		return zero, &ErrWrongType{Name: name, Expected: fmt.Sprintf("%T", zero), Actual: raw}
	}
	return v, nil
}

// GetOrAdd returns the existing value for key, or, if absent, creates one with
// ctor, stores it, and returns it. If an existing value is of the wrong type an
// error is returned.
func GetOrAdd[T any](m Map, key TypedKey[T], ctor func() T) (T, error) {
	raw := m.GetOrAddString(key.Name(), func() any { return ctor() })
	v, ok := raw.(T)
	if !ok {
		var zero T
		return zero, &ErrWrongType{Name: key.Name(), Expected: fmt.Sprintf("%T", zero), Actual: raw}
	}
	return v, nil
}
