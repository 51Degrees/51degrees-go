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
	"testing"
	"time"
)

// readWaitSeconds bounds how long the reads of a finished Flow Data may take
// while the test holds the errors lock. They take no lock, so they finish at
// once.
const readWaitSeconds = 10

// readEveryWay calls every read of a finished Flow Data, and gives a line for
// each answer that is not the one expected.
func readEveryWay(fd FlowData, p Pipeline, a FlowElement) []string {
	var wrong []string
	check := func(ok bool, format string, args ...any) {
		if !ok {
			wrong = append(wrong, fmt.Sprintf(format, args...))
		}
	}
	d, err := fd.Get("a")
	check(err == nil, "Get: %v", err)
	if d != nil {
		v, _ := d.Get("value")
		check(v == "a", "Get gave %v", v)
	}
	again := fd.GetOrAdd("a", func(FlowData) ElementData {
		return NewBaseElementData()
	})
	check(again == d, "GetOrAdd gave another instance")
	check(len(fd.ElementDataAsMap()) == 2, "ElementDataAsMap listed %d",
		len(fd.ElementDataAsMap()))
	check(len(fd.Errors()) == 2, "Errors gave %d", len(fd.Errors()))
	check(fd.HasErrors(), "HasErrors is false")
	v, ok := fd.Evidence().Get("query.supplied")
	check(ok && v == "yes", "Evidence().Get gave %v", v)
	check(len(fd.Evidence().AsMap()) == 1, "Evidence().AsMap listed %d",
		len(fd.Evidence().AsMap()))
	check(len(fd.Evidence().Keys()) == 1, "Evidence().Keys listed %d",
		len(fd.Evidence().Keys()))
	check(fd.Pipeline() == p, "Pipeline gave another Pipeline")
	_, err = Get[*BaseElementData](fd)
	check(err != nil, "Get by type found one of two")
	fromA, err := GetFromElement[*BaseElementData](fd, a)
	check(err == nil && ElementData(fromA) == d, "GetFromElement: %v", err)
	v, ok, err = GetEvidenceOrProperty(fd, "query.supplied")
	check(err == nil && ok && v == "yes", "GetEvidenceOrProperty gave %v, %v",
		v, err)
	check(SuppliedEvidence(fd)["query.supplied"] == "yes",
		"SuppliedEvidence left out query.supplied")
	return wrong
}

// Once processing has finished, no read of the Flow Data takes a lock. The
// test holds the errors lock, the one lock a Flow Data holds, while it calls
// every read, so a read that took it would not finish.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L117-L120
func TestAFinishedFlowDataTakesNoLockToRead(t *testing.T) {
	member := func(key string) FlowElement {
		return newFuncElement(key, nil, func(fd FlowData) error {
			fd.GetOrAdd(key, func(FlowData) ElementData {
				return NewBaseElementData()
			}).Set("value", key)
			fd.AddError(errors.New(key+" recorded"), nil)
			return nil
		})
	}
	a := member("a")
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).
		SetSuppressProcessExceptions(true).
		AddParallel(a, member("b")).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("query.supplied", "yes")
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	base := baseFlowData(fd)
	base.errMu.Lock()
	done := make(chan []string, 1)
	go func() { done <- readEveryWay(fd, p, a) }()
	var wrong []string
	select {
	case wrong = <-done:
		base.errMu.Unlock()
	case <-time.After(readWaitSeconds * time.Second):
		base.errMu.Unlock()
		wrong = <-done
		t.Error("a read of the finished Flow Data waited for the errors lock")
	}
	for _, w := range wrong {
		t.Error(w)
	}
}
