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
	"errors"
	"fmt"
	"sync"
	"testing"
)

// elementAdding is a Flow Element that adds its own Element Data, holding its
// key as a value.
func elementAdding(key string) *funcElement {
	return newFuncElement(key, nil, func(fd FlowData) error {
		d := fd.GetOrAdd(key, func(FlowData) ElementData {
			return NewBaseElementData()
		})
		d.Set("key", key)
		return nil
	})
}

// Element Data is added, found by its key in any case, and listed under its
// key in lower case, and asking again for a key gives the instance already
// added.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/automated-testing.md?plain=1#L79-L83
func TestElementDataIsAddedFoundAndListed(t *testing.T) {
	var again ElementData
	var first ElementData
	last := newFuncElement("last", nil, func(fd FlowData) error {
		first, _ = fd.Get("first")
		again = fd.GetOrAdd("FIRST", func(FlowData) ElementData {
			return NewBaseElementData()
		})
		return nil
	})
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).
		AddFlowElement(elementAdding("First")).
		AddFlowElement(elementAdding("second")).
		AddFlowElement(elementAdding("THIRD")).
		AddFlowElement(last).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"first", "SECOND", "Third"} {
		d, err := fd.Get(key)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if _, err := d.Get("key"); err != nil {
			t.Errorf("%s holds no value: %v", key, err)
		}
	}
	listed := fd.ElementDataAsMap()
	if len(listed) != 3 {
		t.Errorf("listed %d Element Data, want 3: %v", len(listed), listed)
	}
	for _, key := range []string{"first", "second", "third"} {
		if _, ok := listed[key]; !ok {
			t.Errorf("%s is not listed", key)
		}
	}
	if first == nil || again != first {
		t.Error("asking again for an added key did not give the instance added")
	}
}

// Every member of a parallel group adds its own Element Data at the same
// time, and all of them are present afterwards.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/advanced-features/parallel-processing.md?plain=1#L28-L30
func TestEveryMemberOfAParallelGroupAddsItsElementData(t *testing.T) {
	const members = 32
	var ready sync.WaitGroup
	ready.Add(members)
	group := make([]FlowElement, members)
	for i := range group {
		key := fmt.Sprintf("member-%d", i)
		group[i] = newFuncElement(key, nil, func(fd FlowData) error {
			ready.Done()
			ready.Wait()
			d := fd.GetOrAdd(key, func(FlowData) ElementData {
				return NewBaseElementData()
			})
			d.Set("key", key)
			return nil
		})
	}
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).
		AddParallel(group...).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	if n := len(fd.ElementDataAsMap()); n != members {
		t.Errorf("%d Element Data present, want %d", n, members)
	}
	for i := 0; i < members; i++ {
		key := fmt.Sprintf("member-%d", i)
		d, err := fd.Get(key)
		if err != nil {
			t.Errorf("%s: %v", key, err)
			continue
		}
		if v, _ := d.Get("key"); v != key {
			t.Errorf("%s holds %v", key, v)
		}
	}
}

// The members of a parallel group add errors at the same time, and every one
// is present afterwards.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/automated-testing.md?plain=1#L111-L112
func TestMembersOfAParallelGroupAddErrorsAtTheSameTime(t *testing.T) {
	const members, each = 16, 50
	var ready sync.WaitGroup
	ready.Add(members)
	group := make([]FlowElement, members)
	for i := range group {
		group[i] = newFuncElement(fmt.Sprintf("member-%d", i), nil,
			func(fd FlowData) error {
				ready.Done()
				ready.Wait()
				for j := 0; j < each; j++ {
					fd.AddError(errors.New("recorded"), nil)
				}
				return nil
			})
	}
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).
		SetSuppressProcessExceptions(true).AddParallel(group...).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	if n := len(fd.Errors()); n != members*each {
		t.Errorf("%d errors present, want %d", n, members*each)
	}
}

// The errors that members of a parallel group return are added by the group
// once every member has stopped, one for each member that returned one.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/advanced-features/parallel-processing.md?plain=1#L16-L22
func TestAParallelGroupAddsItsMembersErrors(t *testing.T) {
	failing := func(key string) FlowElement {
		return newFuncElement(key, nil, func(FlowData) error {
			return errors.New(key + " failed")
		})
	}
	panicking := newFuncElement("panicking", nil, func(FlowData) error {
		panic("boom")
	})
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).
		SetSuppressProcessExceptions(true).
		AddParallel(failing("a"), elementAdding("good"), failing("b"),
			panicking).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, fe := range fd.Errors() {
		if fe.Element != nil {
			got[fe.Element.DataKey()] = true
		}
	}
	for _, key := range []string{"a", "b", "panicking"} {
		if !got[key] {
			t.Errorf("no error from %s in %v", key, fd.Errors())
		}
	}
	if len(fd.Errors()) != 3 {
		t.Errorf("%d errors, want 3: %v", len(fd.Errors()), fd.Errors())
	}
	if _, err := fd.Get("good"); err != nil {
		t.Errorf("the member that succeeded added nothing: %v", err)
	}
}
