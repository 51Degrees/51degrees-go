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

import "sort"

// EvidenceKeyFilter describes the Evidence keys that a Flow Element (or
// Pipeline) can make use of. All Flow Elements MUST be able to describe their
// accepted Evidence keys programmatically.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/advertize-accepted-evidence.md
type EvidenceKeyFilter interface {
	// Include reports whether the given Evidence key is used by the element.
	Include(key string) bool
}

// WhitelistFilter accepts a fixed set of Evidence keys. This suffices for the
// majority of Flow Elements.
type WhitelistFilter struct {
	keys map[string]struct{}
}

// NewWhitelistFilter creates a filter that accepts exactly the given keys
// (compared case-insensitively).
func NewWhitelistFilter(keys ...string) *WhitelistFilter {
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		set[lower(k)] = struct{}{}
	}
	return &WhitelistFilter{keys: set}
}

// Include reports whether key is in the whitelist.
func (f *WhitelistFilter) Include(key string) bool {
	_, ok := f.keys[lower(key)]
	return ok
}

// Keys returns the whitelisted keys in sorted order.
func (f *WhitelistFilter) Keys() []string {
	out := make([]string, 0, len(f.keys))
	for k := range f.keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// FuncFilter accepts Evidence keys using an arbitrary predicate. This is used
// where listing accepted keys is impractical, for example the Cloud Request
// Engine, which accepts any key but EvidenceHTTPRequest.
type FuncFilter struct {
	fn func(key string) bool
}

// NewFuncFilter creates a filter backed by the given predicate.
func NewFuncFilter(fn func(key string) bool) *FuncFilter { return &FuncFilter{fn: fn} }

// Include applies the predicate.
func (f *FuncFilter) Include(key string) bool { return f.fn(key) }

// AggregateFilter combines multiple filters; a key is included if any of the
// constituent filters includes it. It is used to build the Pipeline-level
// filter from its elements' filters.
type AggregateFilter struct {
	filters []EvidenceKeyFilter
}

// NewAggregateFilter combines the given filters.
func NewAggregateFilter(filters ...EvidenceKeyFilter) *AggregateFilter {
	return &AggregateFilter{filters: filters}
}

// Include reports whether any constituent filter includes the key.
func (f *AggregateFilter) Include(key string) bool {
	for _, sub := range f.filters {
		if sub != nil && sub.Include(key) {
			return true
		}
	}
	return false
}

// KeyLister is implemented by filters that can enumerate the exact keys they
// accept. Function-based filters do not implement it.
type KeyLister interface {
	// Keys returns the exact set of Evidence keys the filter accepts.
	Keys() []string
}

// Keys returns the union of the keys of any constituent filters that can
// enumerate them. Keys from function-based filters cannot be listed and are
// omitted. The result is sorted and de-duplicated.
func (f *AggregateFilter) Keys() []string {
	seen := map[string]struct{}{}
	for _, sub := range f.filters {
		if kl, ok := sub.(KeyLister); ok {
			for _, k := range kl.Keys() {
				seen[k] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
