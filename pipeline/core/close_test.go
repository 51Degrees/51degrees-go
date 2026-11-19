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

import (
	"fmt"
	"testing"
)

// closer is a Flow Data that can be closed.
type closer interface{ Close() error }

// pooledWriter adds Element Data and writes one value to it.
func pooledWriter() *funcElement {
	return newFuncElement("pooled", nil, func(fd FlowData) error {
		fd.GetOrAdd("pooled", func(FlowData) ElementData {
			return NewBaseElementData()
		}).Set("value", true)
		return nil
	})
}

func closeOf(t *testing.T, fd FlowData) closer {
	t.Helper()
	c, ok := fd.(closer)
	if !ok {
		t.Fatalf("%T has no Close", fd)
	}
	return c
}

// Element Data comes from a pool and goes back to it when the Flow Data that
// holds it is closed, so a Pipeline that closes each Flow Data allocates
// less for each request than one that does not.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/resource-cleanup.md?plain=1#L87-L101
func TestClosingAFlowDataReturnsItsElementDataToThePool(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector drops items from sync.Pool at random")
	}
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).
		AddFlowElement(pooledWriter()).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	closeOf(t, p.CreateFlowData())
	run := func(close bool) float64 {
		return testing.AllocsPerRun(200, func() {
			fd := p.CreateFlowData()
			if err := fd.Process(); err != nil {
				t.Fatal(err)
			}
			if close {
				fd.(closer).Close()
			}
		})
	}
	kept, closed := run(false), run(true)
	// The Element Data, its map and the map's storage are reused.
	if closed > kept-3 {
		t.Errorf("a request allocates %v times when its Flow Data is closed "+
			"and %v when it is not, want at least 3 fewer", closed, kept)
	}
}

// Element Data taken from the pool has no values and can be written. Each
// request writes a value under a name of its own, so a value an earlier
// request left in a reused instance shows.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md?plain=1#L87-L96
func TestPooledElementDataStartsEmptyAndOpen(t *testing.T) {
	request := 0
	writer := newFuncElement("pooled", nil, func(fd FlowData) error {
		fd.GetOrAdd("pooled", func(FlowData) ElementData {
			return NewBaseElementData()
		}).Set(fmt.Sprintf("value%d", request), true)
		return nil
	})
	var held map[string]any
	reader := newFuncElement("reader", nil, func(fd FlowData) error {
		d, err := fd.Get("pooled")
		if err != nil {
			return err
		}
		held = d.AsMap()
		return nil
	})
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).
		AddFlowElement(writer).AddFlowElement(reader).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for request = range 20 {
		fd := p.CreateFlowData()
		if err := fd.Process(); err != nil {
			t.Fatalf("request %d: %v", request, err)
		}
		want := fmt.Sprintf("value%d", request)
		if _, ok := held[want]; !ok || len(held) != 1 {
			t.Fatalf("request %d: the Element Data held %v, want only %s",
				request, held, want)
		}
		closeOf(t, fd).Close()
	}
}

// A closed Flow Data holds no Element Data and cannot be processed, and
// closing it again does nothing.
func TestAClosedFlowDataHoldsNothing(t *testing.T) {
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).
		AddFlowElement(pooledWriter()).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	c := closeOf(t, fd)
	if err := c.Close(); err != nil {
		t.Fatalf("Close gave %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("a second Close gave %v", err)
	}
	if _, err := fd.Get("pooled"); err == nil {
		t.Error("a closed Flow Data still gives its Element Data")
	}
	unprocessed := p.CreateFlowData()
	closeOf(t, unprocessed).Close()
	if err := unprocessed.Process(); err == nil {
		t.Error("a closed Flow Data was processed")
	}
}
