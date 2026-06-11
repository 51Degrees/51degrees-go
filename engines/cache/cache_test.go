package cache

import (
	"testing"

	"github.com/51Degrees/pipeline-go/core"
)

func TestLRUEviction(t *testing.T) {
	c := NewLRUCache(2, 1) // single shard, capacity 2
	c.Set("a", 1)
	c.Set("b", 2)
	_, _ = c.Get("a") // touch a so b is least-recently-used
	c.Set("c", 3)     // evicts b
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should have been evicted")
	}
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("a should remain: %v %v", v, ok)
	}
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Fatalf("c should be present: %v %v", v, ok)
	}
}

func TestLRUUpdateExisting(t *testing.T) {
	c := NewLRUCache(4, 1)
	c.Set("a", 1)
	c.Set("a", 2)
	if v, _ := c.Get("a"); v != 2 {
		t.Fatalf("value should be updated, got %v", v)
	}
}

// pipelineless flow data builder for key tests.
func newFlowDataWith(t *testing.T, ev map[string]any) core.FlowData {
	t.Helper()
	p, err := core.NewPipelineBuilder().Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	fd := p.CreateFlowData()
	for k, v := range ev {
		fd.AddEvidence(k, v)
	}
	return fd
}

func TestGenerateKeyDeterministic(t *testing.T) {
	filter := core.NewWhitelistFilter("query.user-agent", "header.user-agent")
	fd1 := newFlowDataWith(t, map[string]any{
		"query.user-agent":  "abc",
		"header.user-agent": "xyz",
		"cookie.irrelevant": "ignored",
	})
	fd2 := newFlowDataWith(t, map[string]any{
		"header.user-agent": "xyz",
		"query.user-agent":  "abc",
	})
	k1 := GenerateKey(fd1, filter)
	k2 := GenerateKey(fd2, filter)
	if k1 != k2 {
		t.Fatalf("keys should match regardless of insertion order:\n%q\n%q", k1, k2)
	}
}

func TestGenerateKeyCaseSensitivity(t *testing.T) {
	filter := core.NewWhitelistFilter("query.user-agent")
	// Key case differs but normalizes; values differ in case and must NOT match.
	fdLowerKey := newFlowDataWith(t, map[string]any{"query.user-agent": "ABC"})
	fdUpperKey := newFlowDataWith(t, map[string]any{"query.User-Agent": "ABC"})
	if GenerateKey(fdLowerKey, filter) != GenerateKey(fdUpperKey, filter) {
		t.Fatal("evidence keys must be compared case-insensitively")
	}

	fdValA := newFlowDataWith(t, map[string]any{"query.user-agent": "abc"})
	fdValB := newFlowDataWith(t, map[string]any{"query.user-agent": "ABC"})
	if GenerateKey(fdValA, filter) == GenerateKey(fdValB, filter) {
		t.Fatal("evidence values must be compared case-sensitively")
	}
}
