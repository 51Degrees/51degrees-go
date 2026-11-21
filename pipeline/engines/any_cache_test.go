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
	"sync"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// mapCache is the simplest cache with the Get and Set of cache.Cache, a map,
// counting how often each is called.
type mapCache struct {
	mu     sync.Mutex
	values map[string]any
	gets   int
	sets   int
}

func (c *mapCache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gets++
	value, ok := c.values[key]
	return value, ok
}

func (c *mapCache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sets++
	c.values[key] = value
}

// An engine uses any cache with Get and Set, not only the package's LRU
// cache, so the same Evidence a second time is answered from that cache.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/caching.md?plain=1#L86-L87
func TestEngineUsesAnyCache(t *testing.T) {
	c := &mapCache{values: make(map[string]any)}
	e := newCountingEngine()
	e.SetCache(c)
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()

	process(t, p, "agentA")
	second := process(t, p, "agentA")
	if got := e.computeCount.Load(); got != 1 {
		t.Errorf("the engine computed %d times for the same Evidence twice, "+
			"want once, the second answered from the cache", got)
	}
	d, err := second.Get("counter")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !d.(AspectData).CacheHit() {
		t.Error("the second result is not marked as a cache hit")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gets != 2 || c.sets != 1 {
		t.Errorf("the cache was asked %d times and given %d values, want "+
			"asked twice and given one", c.gets, c.sets)
	}
}
