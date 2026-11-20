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

package ipi_onpremise

import (
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// The on-premise IP Intelligence Pipeline engine's data cannot be written
// once it has processed.
func TestPipelineEngineDataIsReadOnlyOnceProcessed(t *testing.T) {
	fd := enginePipeline(t).CreateFlowData()
	fd.AddEvidence("query.client-ip", compatibleAddress)
	wantWriteToPanic(t, fd, DataKey)
}

// wantWriteToPanic processes fd, then fails t unless a write to the Element
// Data under key panics with an error saying processing has finished. It
// skips when the pipeline module in use takes any write after processing,
// which Element Data the caller adds before processing shows.
func wantWriteToPanic(t *testing.T, fd core.FlowData, key string) {
	t.Helper()
	probe := fd.GetOrAdd("read-only-probe", func(core.FlowData) core.ElementData {
		return core.NewBaseElementData()
	})
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	if panicked(func() { probe.Set("probe", true) }) == nil {
		t.Skip("the pipeline module in use takes a write after processing")
	}
	d, err := fd.Get(key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	r := panicked(func() { d.Set("written-after-processing", true) })
	if err, ok := r.(error); !ok ||
		!strings.Contains(err.Error(), "finished processing") {
		t.Errorf("a write after processing panicked with %v, want an "+
			"error saying processing has finished", r)
	}
}

// panicked runs f and gives what it panicked with, or nil.
func panicked(f func()) (r any) {
	defer func() { r = recover() }()
	f()
	return nil
}
