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
	"strings"
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

// SetCache attaches a result cache to the Engine. The concrete Engine must
// not attach one where its Aspect Data needs cleanup or is not safe to read
// from several goroutines at once. A cache is also refused, with an error
// logged, while the Engine is in a Pipeline where another Flow Element
// writes its Element Data, because that Flow Element would write into a
// cached instance that other requests are reading. Building such a Pipeline
// with the cache already set fails in the same way.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/caching.md?plain=1#L111-L135
func (b *BaseAspectEngine) SetCache(c cache.Cache) {
	if c != nil {
		for _, p := range b.Pipelines() {
			if err := b.refuseSharedCache(p); err != nil {
				b.Logger().Error(err.Error())
				return
			}
		}
	}
	b.cache = c
}

// OnPipelineAdded refuses a Pipeline in which another Flow Element writes
// the Engine's Element Data while the Engine has a cache.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L234-L237
func (b *BaseAspectEngine) OnPipelineAdded(p core.Pipeline) error {
	if b.cache != nil {
		if err := b.refuseSharedCache(p); err != nil {
			return err
		}
	}
	return b.BaseElement.OnPipelineAdded(p)
}

// refuseSharedCache gives an error when another Flow Element of p, a member
// of a group at any depth included, has the Engine's data key.
func (b *BaseAspectEngine) refuseSharedCache(p core.Pipeline) error {
	if !writtenByAnother(p.FlowElements(), b) {
		return nil
	}
	return core.NewConfigurationError(fmt.Sprintf("the Engine '%s' has a "+
		"cache, so another Flow Element must not write its Element Data, "+
		"and one in the Pipeline does", b.DataKey()), nil)
}

// aspectEngine gives the BaseAspectEngine an Engine embeds, so that the
// Engine can be told apart from another Flow Element with its key.
func (b *BaseAspectEngine) aspectEngine() *BaseAspectEngine { return b }

// writtenByAnother reports whether any of elements, or a member of a group
// among them at any depth, has the data key of b and is not b's Engine.
func writtenByAnother(elements []core.FlowElement, b *BaseAspectEngine) bool {
	for _, e := range elements {
		if own, ok := e.(interface{ aspectEngine() *BaseAspectEngine }); ok &&
			own.aspectEngine() == b {
			continue
		}
		if strings.EqualFold(e.DataKey(), b.DataKey()) {
			return true
		}
		if group, ok := e.(interface{ Children() []core.FlowElement }); ok &&
			writtenByAnother(group.Children(), b) {
			return true
		}
	}
	return false
}

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
		// another Flow Data, and is marked shared so that closing this Flow
		// Data leaves it in the cache rather than giving it to a pool.
		attach(fd, b.DataKey(), data)
		core.MarkShared(data)
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
