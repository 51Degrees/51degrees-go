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

package typedmap

import (
	"errors"
	"sync"
	"testing"
)

func TestTypedKeyRoundTrip(t *testing.T) {
	for _, m := range []Map{NewMap(), NewConcurrentMap()} {
		key := NewTypedKey[int]("answer")
		Add(m, key, 42)
		got, err := Get(m, key)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got != 42 {
			t.Fatalf("got %d, want 42", got)
		}
	}
}

func TestCaseInsensitiveKeys(t *testing.T) {
	m := NewMap()
	m.AddString("Device", "value")
	if v, ok := m.GetString("device"); !ok || v != "value" {
		t.Fatalf("case-insensitive lookup failed: %v %v", v, ok)
	}
	if !m.Contains("DEVICE") {
		t.Fatal("Contains should be case-insensitive")
	}
}

func TestGetNotFound(t *testing.T) {
	m := NewMap()
	_, err := GetByName[int](m, "missing")
	var nf *ErrNotFound
	if !errors.As(err, &nf) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGetWrongType(t *testing.T) {
	m := NewMap()
	m.AddString("k", "a string")
	_, err := GetByName[int](m, "k")
	var wt *ErrWrongType
	if !errors.As(err, &wt) {
		t.Fatalf("want ErrWrongType, got %v", err)
	}
}

func TestGetOrAdd(t *testing.T) {
	m := NewMap()
	key := NewTypedKey[[]string]("list")
	calls := 0
	ctor := func() []string { calls++; return []string{"x"} }

	v1, err := GetOrAdd(m, key, ctor)
	if err != nil || len(v1) != 1 {
		t.Fatalf("first GetOrAdd: %v %v", v1, err)
	}
	v2, err := GetOrAdd(m, key, ctor)
	if err != nil || len(v2) != 1 {
		t.Fatalf("second GetOrAdd: %v %v", v2, err)
	}
	if calls != 1 {
		t.Fatalf("ctor called %d times, want 1", calls)
	}
}

func TestConcurrentMapRace(t *testing.T) {
	m := NewConcurrentMap()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.AddString("k", i)
			_, _ = m.GetString("k")
			_ = m.Keys()
			_ = m.AsMap()
		}(i)
	}
	wg.Wait()
}

func TestRemove(t *testing.T) {
	m := NewConcurrentMap()
	m.AddString("k", 1)
	m.Remove("K")
	if m.Contains("k") {
		t.Fatal("Remove should be case-insensitive")
	}
}
