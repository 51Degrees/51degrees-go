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

package engines

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/cache"
)

// goroutineID gives the number the runtime prints for the calling goroutine.
func goroutineID() string {
	buf := make([]byte, 64)
	buf = buf[:runtime.Stack(buf, false)]
	return string(bytes.Fields(buf)[1])
}

// callerEngine counts the times its work ran on a goroutine other than the
// one that called Process.
type callerEngine struct {
	*BaseAspectEngine
	delay     time.Duration
	computed  atomic.Int64
	elsewhere atomic.Int64
}

func newCallerEngine(delay time.Duration) *callerEngine {
	return &callerEngine{
		BaseAspectEngine: NewBaseAspectEngine("caller",
			core.NewWhitelistFilter("query.user-agent"), nil),
		delay: delay,
	}
}

func (e *callerEngine) Process(fd core.FlowData) error {
	caller := goroutineID()
	return e.Execute(fd, func() (AspectData, error) {
		e.computed.Add(1)
		if goroutineID() != caller {
			e.elsewhere.Add(1)
		}
		time.Sleep(e.delay)
		d := newCountingData()
		ua, _ := fd.Evidence().Get("query.user-agent")
		d.Set("ua", ua)
		return d, nil
	})
}

// Lazy loading is deprecated and setting it has no effect, so an Engine
// processes on the goroutine that processes the Flow Data and gives the same
// values whatever is set.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/advanced-features/lazy-loading.md?plain=1#L3-L9
// and https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/automated-testing.md?plain=1#L149-L151
func TestLazyLoadingChangesNothing(t *testing.T) {
	const delay = 20 * time.Millisecond
	for _, c := range []struct {
		name    string
		set     bool
		enabled bool
		timeout time.Duration
	}{
		{"not set", false, false, 0},
		{"on with no timeout", true, true, 0},
		{"on with a timeout longer than the work", true, true, time.Second},
		{"on with a timeout shorter than the work", true, true,
			time.Millisecond},
		{"off with a timeout", true, false, time.Millisecond},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := newCallerEngine(delay)
			if c.set {
				e.SetLazyLoading(c.enabled, c.timeout)
			}
			p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			defer p.Close()
			fd := p.CreateFlowData()
			fd.AddEvidence("query.user-agent", "agent")
			if err := fd.Process(); err != nil {
				t.Fatalf("process: %v", err)
			}
			d, err := fd.Get("caller")
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if _, ok := d.(*countingData); !ok {
				t.Errorf("the Flow Data holds %T, want the Engine's own "+
					"*engines.countingData", d)
			}
			if v, err := d.Get("ua"); err != nil || v != "agent" {
				t.Errorf("ua is %v with error %v, want agent and no error",
					v, err)
			}
			if got := e.computed.Load(); got != 1 {
				t.Errorf("the Engine worked %d times, want once", got)
			}
			if got := e.elsewhere.Load(); got != 0 {
				t.Errorf("the Engine worked on another goroutine %d times, "+
					"want none", got)
			}
		})
	}
}

// Setting lazy loading leaves a cache the Engine has in use.
func TestLazyLoadingLeavesTheCacheInUse(t *testing.T) {
	e := newCallerEngine(0)
	e.SetCache(cache.NewLRUCache(100, 4))
	e.SetLazyLoading(true, time.Second)
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	var last core.ElementData
	for range 2 {
		fd := p.CreateFlowData()
		fd.AddEvidence("query.user-agent", "agent")
		if err := fd.Process(); err != nil {
			t.Fatalf("process: %v", err)
		}
		if last, err = fd.Get("caller"); err != nil {
			t.Fatalf("get: %v", err)
		}
		if v, err := last.Get("ua"); err != nil || v != "agent" {
			t.Fatalf("ua is %v with error %v, want agent and no error",
				v, err)
		}
	}
	if got := e.computed.Load(); got != 1 {
		t.Errorf("the Engine worked %d times for the same Evidence twice, "+
			"want once, with the second served from the cache", got)
	}
	if !last.(AspectData).CacheHit() {
		t.Error("the second Flow Data was not served from the cache")
	}
}

// The doc comment of SetLazyLoading says it is deprecated and has no effect,
// and points to lazy properties.
func TestSetLazyLoadingIsDocumentedAsDeprecated(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "aspectengine.go", nil,
		parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var doc string
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "SetLazyLoading" {
			doc = fn.Doc.Text()
		}
	}
	for _, want := range []string{"\nDeprecated: ", "no effect",
		"lazy-properties"} {
		if !strings.Contains("\n"+doc, want) {
			t.Errorf("the doc comment of SetLazyLoading does not contain "+
				"%q, in:\n%s", strings.TrimSpace(want), doc)
		}
	}
}
