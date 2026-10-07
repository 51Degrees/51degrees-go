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

package engines

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/engines/cache"
)

// AspectEngine is a specialization of Flow Element with the features common to
// 51Degrees Engines: caching, lazy loading, a data-source tier, and missing
// Property reporting.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/conceptual-overview.md#aspect-engine
type AspectEngine interface {
	core.FlowElement
	// DataSourceTier returns the name of the data tier/source currently in use
	// (for example "lite" or "cloud"). Used by the missing-Property feature.
	DataSourceTier() string
}

// BaseAspectEngine supplies the shared Engine behavior that concrete engines
// embed: optional result caching and optional lazy loading. Concrete engines
// implement their processing logic in a compute function passed to Execute.
type BaseAspectEngine struct {
	*core.BaseElement
	cache       cache.Cache
	lazyLoading bool
	lazyTimeout time.Duration
	tier        string
	warnOnce    sync.Once
}

// NewBaseAspectEngine creates a BaseAspectEngine with the given data key and
// evidence filter.
func NewBaseAspectEngine(dataKey string, filter core.EvidenceKeyFilter, logger *slog.Logger) *BaseAspectEngine {
	return &BaseAspectEngine{BaseElement: core.NewBaseElement(dataKey, filter, logger)}
}

// SetCache attaches a result cache to the Engine. A cache MUST NOT be attached
// to an Engine whose Aspect Data requires resource cleanup or is not
// thread-safe; that is the responsibility of the concrete engine.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/caching.md#data-lifetime-and-concurrency-issues
func (b *BaseAspectEngine) SetCache(c cache.Cache) { b.cache = c }

// SetLazyLoading enables lazy loading with the given access timeout. When
// enabled, Execute returns immediately and the compute function runs in the
// background; accessing a Property value blocks until it completes (or the
// timeout elapses). A timeout of zero means wait indefinitely.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/advanced-features/lazy-loading.md
func (b *BaseAspectEngine) SetLazyLoading(enabled bool, timeout time.Duration) {
	b.lazyLoading = enabled
	b.lazyTimeout = timeout
}

// SetDataSourceTier sets the data-source tier name.
func (b *BaseAspectEngine) SetDataSourceTier(tier string) { b.tier = tier }

// DataSourceTier returns the data-source tier name.
func (b *BaseAspectEngine) DataSourceTier() string { return b.tier }

// Execute runs an Engine's processing with caching and lazy loading applied as
// configured, then stores the resulting Aspect Data in the Flow Data under the
// Engine's data key. Concrete engines call this from their Process method,
// supplying a compute function that performs the actual work.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/caching.md#process-flow
func (b *BaseAspectEngine) Execute(fd core.FlowData, compute func() (AspectData, error)) error {
	if b.lazyLoading {
		if b.cache != nil {
			// Lazy loading and caching are mutually exclusive here: the lazy
			// path does not consult the cache. Warn once so the misconfiguration
			// is visible rather than silent.
			b.warnOnce.Do(func() {
				b.Logger().Warn("engine has both lazy loading and a cache configured; "+
					"the cache is ignored while lazy loading is enabled", "engine", b.DataKey())
			})
		}
		return b.executeLazy(fd, compute)
	}

	if b.cache != nil {
		key := cache.GenerateKey(fd, b.EvidenceKeyFilter())
		if v, ok := b.cache.Get(key); ok {
			if data, ok := v.(AspectData); ok && data != nil {
				// Do not mutate the shared cached instance; report the cache hit
				// through a per-Flow-Data view so the original producer's result
				// is unaffected.
				b.store(fd, cacheHitView{AspectData: data})
				return nil
			}
		}
		data, err := compute()
		if err != nil {
			return err
		}
		if data == nil {
			return core.NewDataError("engine compute returned no data and no error")
		}
		b.cache.Set(key, data)
		b.store(fd, data)
		return nil
	}

	data, err := compute()
	if err != nil {
		return err
	}
	if data == nil {
		return core.NewDataError("engine compute returned no data and no error")
	}
	b.store(fd, data)
	return nil
}

// cacheHitView is a per-Flow-Data view over a cached AspectData that reports a
// cache hit without mutating the shared cached instance.
type cacheHitView struct{ AspectData }

// CacheHit always reports true for a value served from the cache.
func (c cacheHitView) CacheHit() bool { return true }

// UnwrapElementData lets type-based access (core.Get) reach the underlying
// concrete Aspect Data behind the view.
func (c cacheHitView) UnwrapElementData() core.ElementData { return c.AspectData }

func (b *BaseAspectEngine) store(fd core.FlowData, data AspectData) {
	fd.GetOrAdd(b.DataKey(), func(core.FlowData) core.ElementData { return data })
}

func (b *BaseAspectEngine) executeLazy(fd core.FlowData, compute func() (AspectData, error)) error {
	lz := &lazyAspectData{done: make(chan struct{}), timeout: b.lazyTimeout}
	b.store(fd, lz)
	go func() {
		defer close(lz.done)
		defer func() {
			if r := recover(); r != nil {
				lz.err = fmt.Errorf("panic during lazy engine processing: %v", r)
			}
		}()
		lz.data, lz.err = compute()
	}()
	return nil
}

// lazyAspectData defers computation of an Engine's Aspect Data. Property access
// blocks until the background computation completes or the timeout elapses.
type lazyAspectData struct {
	done    chan struct{}
	timeout time.Duration
	data    AspectData
	err     error
}

func (l *lazyAspectData) wait() error {
	// Fast path: already complete (the common case once computed), avoiding any
	// timer allocation.
	select {
	case <-l.done:
		return l.err
	default:
	}
	if l.timeout > 0 {
		timer := time.NewTimer(l.timeout)
		defer timer.Stop()
		select {
		case <-l.done:
		case <-timer.C:
			return core.NewDataError("timed out waiting for lazy-loaded engine result")
		}
	} else {
		<-l.done
	}
	return l.err
}

func (l *lazyAspectData) Get(propertyName string) (any, error) {
	if err := l.wait(); err != nil {
		return nil, err
	}
	return l.data.Get(propertyName)
}

func (l *lazyAspectData) Set(propertyName string, value any) {
	// The background compute function builds and populates its own Aspect Data;
	// external Set calls on the lazy wrapper are not expected and are ignored.
}

// AsMap returns the computed values, or an empty map if the lazy load timed out
// or failed. These accessors cannot return an error, so a failed lazy load is
// indistinguishable here from one that legitimately produced nothing; use Get,
// which does surface the wait error, to tell them apart.
func (l *lazyAspectData) AsMap() map[string]any {
	if err := l.wait(); err != nil {
		return map[string]any{}
	}
	return l.data.AsMap()
}

// CacheHit reports the cached state, or false if the lazy load timed out or
// failed. See the note on AsMap about the discarded error.
func (l *lazyAspectData) CacheHit() bool {
	if err := l.wait(); err != nil {
		return false
	}
	return l.data.CacheHit()
}

func (l *lazyAspectData) setCacheHit(v bool) {
	if err := l.wait(); err == nil {
		l.data.setCacheHit(v)
	}
}
