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

package usagesharing

// These tests follow the Package elements of the usage sharing record.
// See https://github.com/51Degrees/specifications/blob/e36d54620caa31745059208b9c09d0ae91e86f05/pipeline-specification/pipeline-elements/usage-sharing-element.md?plain=1#L199-L207

import (
	"reflect"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

const (
	pipelineModule = "github.com/51Degrees/51degrees-go/pipeline/v4"
	ipiModule      = "github.com/51Degrees/51degrees-go/ip-intelligence/v4"
	commonModule   = "github.com/51Degrees/51degrees-go/common/v4"
)

// The packages are the 51Degrees modules the program was built with, each
// once and in path order, with the version Go gives the one in use. A
// module replaced by another release gives that release, and one replaced
// by a folder gives (devel), as Go gives a module built from its folder.
func TestPackagesAreThe51DegreesModulesBuiltIn(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Path: "example.com/app", Version: "(devel)"},
		Deps: []*debug.Module{
			{Path: pipelineModule, Version: "v4.5.174"},
			{Path: "github.com/fsnotify/fsnotify", Version: "v1.9.0"},
			{Path: ipiModule, Version: "v4.5.173",
				Replace: &debug.Module{Path: ipiModule, Version: "v4.5.174"}},
			{Path: commonModule, Version: "v4.5.173",
				Replace: &debug.Module{Path: "../common"}},
		},
	}
	want := []packageVersion{
		{commonModule, "(devel)"},
		{ipiModule, "v4.5.174"},
		{pipelineModule, "v4.5.174"},
	}
	if got := packagesIn(info); !reflect.DeepEqual(got, want) {
		t.Errorf("packages %v, want %v", got, want)
	}
}

// The program's own module is not a package it uses, even when it is a
// 51Degrees module such as an example.
func TestTheProgramIsNotAPackageItUses(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{
			Path:    "github.com/51Degrees/51degrees-go/examples/pipeline",
			Version: "(devel)"},
		Deps: []*debug.Module{{Path: pipelineModule, Version: "(devel)"}},
	}
	want := []packageVersion{{pipelineModule, "(devel)"}}
	if got := packagesIn(info); !reflect.DeepEqual(got, want) {
		t.Errorf("packages %v, want %v", got, want)
	}
}

// Each record names every 51Degrees module in use and its version, after
// the FlowElement elements and before Language, as the specification's
// record does.
func TestEachRecordNamesEveryPackageInUse(t *testing.T) {
	c := &capture{}
	srv := c.server()
	defer srv.Close()
	b := NewBuilder().SetShareUsageURL(srv.URL).SetMinimumEntriesPerMessage(1)
	b.packages = func() []packageVersion {
		return []packageVersion{
			{ipiModule, "v4.5.174"}, {pipelineModule, "v4.5.174"}}
	}
	p, err := core.NewPipelineBuilder().AddFlowElement(b.Build()).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	c.waitFor(t, 1)
	c.mu.Lock()
	body := string(c.bodies[0])
	c.mu.Unlock()
	want := "</FlowElement>" +
		`<Package Name="` + ipiModule + `">v4.5.174</Package>` +
		`<Package Name="` + pipelineModule + `">v4.5.174</Package>` +
		"<Language>Go</Language>"
	if !strings.Contains(body, want) {
		t.Errorf("the record does not hold %s in %s", want, body)
	}
}
