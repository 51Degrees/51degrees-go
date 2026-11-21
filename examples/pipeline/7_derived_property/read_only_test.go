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

package main

import (
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// The example's session element's data cannot be written once it has
// processed.
func TestSessionElementDataIsReadOnlyOnceProcessed(t *testing.T) {
	e, err := newSessionElement(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantWriteToPanic(t, processOnce(t, e), sessionKey)
}

// wantWriteToPanic fails t unless a write to the Element Data under key
// panics with an error saying processing has finished.
func wantWriteToPanic(t *testing.T, fd core.FlowData, key string) {
	t.Helper()
	d, err := fd.Get(key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() {
		r := recover()
		err, ok := r.(error)
		if !ok || !strings.Contains(err.Error(), "finished processing") {
			t.Errorf("a write after processing panicked with %v, want an "+
				"error saying processing has finished", r)
		}
	}()
	d.Set("written-after-processing", true)
}

// processOnce builds a Pipeline of the elements, processes one date of
// birth and gives the Flow Data.
func processOnce(t *testing.T, elements ...core.FlowElement) core.FlowData {
	t.Helper()
	b := core.NewPipelineBuilder()
	for _, e := range elements {
		b.AddFlowElement(e)
	}
	p, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	fd := p.CreateFlowData()
	fd.AddEvidence("query.date-of-birth", "1992-04-15")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	return fd
}
