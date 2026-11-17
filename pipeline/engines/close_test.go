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
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/cache"
)

// Aspect Data a cache holds is shared with other requests, so it never goes
// back to a pool when the Flow Data that made it is closed.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/resource-cleanup.md?plain=1#L89-L96
func TestCachedAspectDataNeverGoesBackToAPool(t *testing.T) {
	e := newCountingEngine()
	e.SetCache(cache.NewLRUCache(100, 4))
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	closeAll := func(ua string) core.ElementData {
		fd := process(t, p, ua)
		d, err := fd.Get("counter")
		if err != nil {
			t.Fatal(err)
		}
		c, ok := fd.(interface{ Close() error })
		if !ok {
			t.Fatalf("%T has no Close", fd)
		}
		c.Close()
		return d
	}
	cached := closeAll("agentA")
	for _, ua := range []string{"agentB", "agentC", "agentD"} {
		closeAll(ua)
	}
	if v, err := cached.Get("ua"); err != nil || v != "agentA" {
		t.Errorf("the cached Aspect Data gives ua %v with error %v, want "+
			"agentA", v, err)
	}
	fd := process(t, p, "agentA")
	d, _ := fd.Get("counter")
	if v, err := d.Get("ua"); err != nil || v != "agentA" {
		t.Errorf("the cache gives ua %v with error %v, want agentA", v, err)
	}
	if got := e.computeCount.Load(); got != 4 {
		t.Errorf("the Engine worked %d times, want 4", got)
	}
}
