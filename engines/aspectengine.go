package engines

import (
	"fmt"
	"log/slog"
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
		return b.executeLazy(fd, compute)
	}

	if b.cache != nil {
		key := cache.GenerateKey(fd, b.EvidenceKeyFilter())
		if v, ok := b.cache.Get(key); ok {
			data := v.(AspectData)
			data.setCacheHit(true)
			b.store(fd, data)
			return nil
		}
		data, err := compute()
		if err != nil {
			return err
		}
		b.cache.Set(key, data)
		b.store(fd, data)
		return nil
	}

	data, err := compute()
	if err != nil {
		return err
	}
	b.store(fd, data)
	return nil
}

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

func (l *lazyAspectData) AsMap() map[string]any {
	if err := l.wait(); err != nil {
		return map[string]any{}
	}
	return l.data.AsMap()
}

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
