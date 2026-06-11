// Package cache provides the result cache used by Aspect Engines, together with
// deterministic cache-key generation from Evidence.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/caching.md
package cache

import (
	"container/list"
	"fmt"
	"hash/fnv"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/51Degrees/pipeline-go/core"
)

// Default cache configuration values from the caching specification.
const (
	DefaultSize = 1000
)

// Cache is the simple interface an Aspect Engine cache must satisfy. Any
// conforming cache may be supplied.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/caching.md#cache-implementation
type Cache interface {
	// Get returns the cached value for key and whether it was present.
	Get(key string) (any, bool)
	// Set stores value under key.
	Set(key string, value any)
}

// LRUCache is a sharded least-recently-used cache. Sharding bounds memory use
// and keeps contention low under concurrent access, as recommended by the
// specification.
type LRUCache struct {
	shards []*shard
	mask   uint32
}

// NewLRUCache creates a sharded LRU cache that holds up to size items in total,
// split across concurrency shards. If size <= 0, DefaultSize is used. If
// concurrency <= 0, the number of CPU cores is used. The shard count is rounded
// up to the next power of two so a fast bit-mask can select the shard.
func NewLRUCache(size, concurrency int) *LRUCache {
	if size <= 0 {
		size = DefaultSize
	}
	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
	}
	shardCount := nextPow2(concurrency)
	perShard := size / shardCount
	if perShard < 1 {
		perShard = 1
	}
	c := &LRUCache{
		shards: make([]*shard, shardCount),
		mask:   uint32(shardCount - 1),
	}
	for i := range c.shards {
		c.shards[i] = newShard(perShard)
	}
	return c
}

func (c *LRUCache) shardFor(key string) *shard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return c.shards[h.Sum32()&c.mask]
}

// Get returns the cached value for key.
func (c *LRUCache) Get(key string) (any, bool) { return c.shardFor(key).get(key) }

// Set stores value under key.
func (c *LRUCache) Set(key string, value any) { c.shardFor(key).set(key, value) }

type entry struct {
	key   string
	value any
}

type shard struct {
	mu       sync.Mutex
	capacity int
	ll       *list.List
	items    map[string]*list.Element
}

func newShard(capacity int) *shard {
	return &shard{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[string]*list.Element),
	}
}

func (s *shard) get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if el, ok := s.items[key]; ok {
		s.ll.MoveToFront(el)
		return el.Value.(*entry).value, true
	}
	return nil, false
}

func (s *shard) set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if el, ok := s.items[key]; ok {
		s.ll.MoveToFront(el)
		el.Value.(*entry).value = value
		return
	}
	el := s.ll.PushFront(&entry{key: key, value: value})
	s.items[key] = el
	if s.ll.Len() > s.capacity {
		oldest := s.ll.Back()
		if oldest != nil {
			s.ll.Remove(oldest)
			delete(s.items, oldest.Value.(*entry).key)
		}
	}
}

func nextPow2(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

// GenerateKey builds a deterministic cache key from the Evidence in fd that is
// accepted by filter. Evidence keys are compared case-insensitively and ordered
// using the defined Evidence precedence so the key is stable across requests;
// Evidence values are compared case-sensitively.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/caching.md#generation-of-keys
func GenerateKey(fd core.FlowData, filter core.EvidenceKeyFilter) string {
	all := fd.Evidence().AsMap()
	keys := make([]string, 0, len(all))
	for k := range all {
		if filter == nil || filter.Include(k) {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		pi, fi := core.SplitEvidenceKey(keys[i])
		pj, fj := core.SplitEvidenceKey(keys[j])
		if c := core.ComparePrefixes(pi, pj); c != 0 {
			return c < 0
		}
		return fi < fj
	})
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k) // already normalized to lower case by Evidence
		b.WriteByte('\x1f')
		b.WriteString(valueString(all[k]))
		b.WriteByte('\x1e')
	}
	return b.String()
}

func valueString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}
