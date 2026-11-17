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
	"log/slog"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/cache"
)

// AspectEngine is a specialization of Flow Element with the features common to
// 51Degrees Engines: caching, a data-source tier, and missing Property
// reporting.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/conceptual-overview.md#aspect-engine
type AspectEngine interface {
	core.FlowElement
	// DataSourceTier returns the name of the data tier/source currently in use
	// (for example "lite" or "cloud"). Used by the missing-Property feature.
	DataSourceTier() string
}

// BaseAspectEngine supplies the shared Engine behavior that concrete engines
// embed: optional result caching. Concrete engines implement their processing
// logic in a compute function passed to Execute.
type BaseAspectEngine struct {
	*core.BaseElement
	cache cache.Cache
	tier  string
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

// SetLazyLoading has no effect. The Engine processes on the goroutine that
// processes the Flow Data and gives the same values whatever is set here,
// with or without a timeout.
//
// Deprecated: lazy loading has no effect and is kept so that existing code
// still builds. Long-running work in a Flow Element belongs in a lazy
// property of its own Element Data, as
// https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/thread-safety.md#lazy-properties
// describes.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/advanced-features/lazy-loading.md
func (b *BaseAspectEngine) SetLazyLoading(enabled bool, timeout time.Duration) {
}

// SetDataSourceTier sets the data-source tier name.
func (b *BaseAspectEngine) SetDataSourceTier(tier string) { b.tier = tier }

// DataSourceTier returns the data-source tier name.
func (b *BaseAspectEngine) DataSourceTier() string { return b.tier }

// Execute runs an Engine's processing on the calling goroutine, with caching
// applied as configured, then stores the resulting Aspect Data in the Flow Data under the
// Engine's data key. Concrete engines call this from their Process method,
// supplying a compute function that performs the actual work.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/caching.md#process-flow
func (b *BaseAspectEngine) Execute(fd core.FlowData, compute func() (AspectData, error)) error {
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
		// The data is told its Engine before the cache can serve it to
		// another Flow Data.
		attach(fd, b.DataKey(), data)
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

// attach tells data the Flow Data and key it is held under, where the Flow
// Data holds a view of data or has not stored it yet.
func attach(fd core.FlowData, key string, data core.ElementData) {
	if added, ok := data.(core.FlowDataAdded); ok {
		added.AddedToFlowData(fd, key, data)
	}
}

func (b *BaseAspectEngine) store(fd core.FlowData, data AspectData) {
	fd.GetOrAdd(b.DataKey(), func(core.FlowData) core.ElementData { return data })
}
