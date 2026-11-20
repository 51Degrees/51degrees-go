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

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
)

// Each name the specification gives a Performance Profile is read, in any
// case, and any other name is refused with the names that can be used.
func TestParsePerformanceProfile(t *testing.T) {
	for name, want := range map[string]ipi_interop.PerformanceProfile{
		"LowMemory":       ipi_interop.LowMemory,
		"Balanced":        ipi_interop.Balanced,
		"BalancedTemp":    ipi_interop.BalancedTemp,
		"HighPerformance": ipi_interop.HighPerformance,
		"MaxPerformance":  ipi_interop.InMemory,
		"InMemory":        ipi_interop.InMemory,
		" lowmemory ":     ipi_interop.LowMemory,
	} {
		if got, err := ParsePerformanceProfile(name); err != nil || got != want {
			t.Errorf("%q is read as %v, %v, want %v", name, got, err, want)
		}
	}
	_, err := ParsePerformanceProfile("Fastest")
	if err == nil || !strings.Contains(err.Error(), "MaxPerformance") {
		t.Errorf("an unknown name gave %v, want an error that lists the "+
			"names", err)
	}
}

// ReuseOperationalFile turned on is refused, in code and in a configuration
// file, and turned off is accepted.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L621
func TestReuseOperationalFileCanOnlyBeOff(t *testing.T) {
	if p, err := NewPipelineEngine(testEngineOptions(t,
		WithReuseOperationalFile(true))...); err == nil {
		p.Close()
		t.Error("NewPipelineEngine took ReuseOperationalFile turned on")
	}
	newTestPipelineEngine(t, WithReuseOperationalFile(false))

	pipeline, err := buildFromFile(t, configurationFile(t, BuilderName,
		dataFileParameter(t)+`, "AutoUpdate": false, "ReuseOperationalFile": true`))
	if err == nil {
		pipeline.Close()
		t.Error("the Pipeline was built with ReuseOperationalFile turned on")
	} else if !strings.Contains(err.Error(), "ReuseOperationalFile") {
		t.Errorf("the error %q does not name ReuseOperationalFile", err)
	}
	pipeline, err = buildFromFile(t, configurationFile(t, BuilderName,
		dataFileParameter(t)+`, "AutoUpdate": false, "ReuseOperationalFile": false`))
	if err != nil {
		t.Fatalf("ReuseOperationalFile turned off gave %v", err)
	}
	pipeline.Close()
}