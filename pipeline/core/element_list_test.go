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
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
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

// lateAddWaitMilliseconds is how long the slow member of a group waits, once
// the other members have returned, before it reads the errors. A group that
// added each member's error as the member returned would have added both by
// then. A group that adds them once every member has stopped passes however
// long the wait is.
const lateAddWaitMilliseconds = 100

// A parallel group adds the errors its members return, and their panics, only
// once every member has stopped, in the order the members were added to the
// group. The slow member is added first and returns last, and sees no error
// while it runs although the other two members have already failed.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/advanced-features/parallel-processing.md?plain=1#L37-L40
func TestAParallelGroupAddsItsMembersErrorsOnceAllHaveStopped(t *testing.T) {
	var others sync.WaitGroup
	others.Add(2)
	seen := -1
	slow := newFuncElement("slow", nil, func(fd FlowData) error {
		others.Wait()
		time.Sleep(lateAddWaitMilliseconds * time.Millisecond)
		seen = len(fd.Errors())
		return errors.New("slow failed")
	})
	fast := newFuncElement("fast", nil, func(FlowData) error {
		defer others.Done()
		return errors.New("fast failed")
	})
	panicking := newFuncElement("panicking", nil, func(FlowData) error {
		defer others.Done()
		panic("boom")
	})
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).
		SetSuppressProcessExceptions(true).
		AddParallel(slow, fast, panicking).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	if seen != 0 {
		t.Errorf("the slow member saw %d errors while it ran, want 0", seen)
	}
	want := []string{"slow", "fast", "panicking"}
	var got []string
	for _, fe := range fd.Errors() {
		key := ""
		if fe.Element != nil {
			key = fe.Element.DataKey()
		}
		got = append(got, key)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("errors are from %v, want %v", got, want)
	}
	if msg := fd.Errors()[2].Err.Error(); !strings.Contains(msg, "boom") {
		t.Errorf("the panic was added as %q", msg)
	}
}

// listedKeys gives the keys in a Flow Data's list of Element Data, from the
// head.
func listedKeys(fd *flowData) []string {
	var keys []string
	for n := fd.head.Load(); n != nil; n = n.next {
		keys = append(keys, n.key)
	}
	return keys
}

// addWaitSeconds bounds how long a test waits for adds that take no lock and
// so finish at once.
const addWaitSeconds = 10

// A member of a parallel group that read the head of the list before another
// member added a node puts its own node on the new head and tries again, so
// both are kept, the newest first. The first member makes its Element Data
// only once the other member's add is done, which an add made under a lock
// held for the whole add would not allow. The test holds the errors lock
// while the group runs, so an add made under that lock would not finish
// either. The test reads the list itself, so a Flow Data that kept its
// Element Data in a map would not pass it.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L92-L103
func TestAMemberAddsOnceAnotherMemberHasChangedTheHead(t *testing.T) {
	headRead := make(chan struct{})
	otherAdded := make(chan struct{})
	giveUp := make(chan struct{})
	first := newFuncElement("first", nil, func(fd FlowData) error {
		fd.GetOrAdd("first", func(FlowData) ElementData {
			close(headRead)
			select {
			case <-otherAdded:
			case <-giveUp:
			}
			return NewBaseElementData()
		})
		return nil
	})
	second := newFuncElement("second", nil, func(fd FlowData) error {
		<-headRead
		fd.GetOrAdd("second", func(FlowData) ElementData {
			return NewBaseElementData()
		})
		close(otherAdded)
		return nil
	})
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).
		AddParallel(first, second).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	base := baseFlowData(fd)
	base.errMu.Lock()
	done := make(chan error, 1)
	go func() { done <- fd.Process() }()
	select {
	case err = <-done:
		base.errMu.Unlock()
	case <-time.After(addWaitSeconds * time.Second):
		base.errMu.Unlock()
		close(giveUp)
		err = <-done
		t.Error("adding Element Data in a group waited for a lock")
	}
	if err != nil {
		t.Fatal(err)
	}
	if got := listedKeys(base); !slices.Equal(got, []string{"first", "second"}) {
		t.Errorf("the list holds %v, want [first second]", got)
	}
}

// The errors lock is the only lock a Flow Data holds, so Element Data is
// added with none.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L92-L103
func TestTheErrorsLockIsTheOnlyFlowDataLock(t *testing.T) {
	locker := reflect.TypeOf((*sync.Locker)(nil)).Elem()
	fields := reflect.TypeOf((*flowData)(nil)).Elem()
	for i := 0; i < fields.NumField(); i++ {
		f := fields.Field(i)
		if f.Name != "errMu" && reflect.PointerTo(f.Type).Implements(locker) {
			t.Errorf("flowData.%s is a lock", f.Name)
		}
	}
}
