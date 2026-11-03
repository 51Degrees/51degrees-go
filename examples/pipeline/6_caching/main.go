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

// Example 6: using a result cache with an Aspect Engine.
//
// The engine simulates expensive work with a short delay. With a cache attached,
// repeated requests for the same Evidence are served from the cache rather than
// recomputed, which this example demonstrates by counting computations and
// timing repeated lookups.
//
// Run with: go run ./examples/6_caching
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/required-examples.md
package main

import (
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/cache"
	"github.com/51Degrees/51degrees-go/examples/pipeline/starsign"
)

const evidenceDateOfBirth = "query.date-of-birth"

// CachedStarSignEngine is an Aspect Engine with (simulated) expensive processing.
type CachedStarSignEngine struct {
	*engines.BaseAspectEngine
	computeCount atomic.Int64
}

// StarSignData is the Aspect Data produced by the engine.
type StarSignData struct {
	*engines.BaseAspectData
}

// NewCachedStarSignEngine creates the engine with a cache attached.
func NewCachedStarSignEngine() *CachedStarSignEngine {
	e := &CachedStarSignEngine{BaseAspectEngine: engines.NewBaseAspectEngine(
		"starsign", core.NewWhitelistFilter(evidenceDateOfBirth), nil)}
	e.SetProperties([]core.PropertyMetadata{{Name: "starsign", Element: e, Available: true}})
	e.SetCache(cache.NewLRUCache(cache.DefaultSize, 0))
	return e
}

// Process computes the star sign, simulating expensive work that the cache will
// avoid repeating.
func (e *CachedStarSignEngine) Process(fd core.FlowData) error {
	return e.Execute(fd, func() (engines.AspectData, error) {
		e.computeCount.Add(1)
		time.Sleep(20 * time.Millisecond) // simulate expensive work
		data := &StarSignData{BaseAspectData: engines.NewBaseAspectData()}
		sign := "Unknown"
		if raw, ok := fd.Evidence().Get(evidenceDateOfBirth); ok {
			if dob, err := starsign.ParseDate(fmt.Sprintf("%v", raw)); err == nil {
				sign = starsign.SignForDate(dob.Month(), dob.Day())
			}
		}
		data.Set("starsign", sign)
		return data, nil
	})
}

func main() {
	engine := NewCachedStarSignEngine()
	pipeline, err := core.NewPipelineBuilder().AddFlowElement(engine).Build()
	if err != nil {
		log.Fatal(err)
	}
	defer pipeline.Close()

	lookup := func(dob string) (string, bool, time.Duration) {
		start := time.Now()
		fd := pipeline.CreateFlowData()
		fd.AddEvidence(evidenceDateOfBirth, dob)
		if err := fd.Process(); err != nil {
			log.Fatal(err)
		}
		data, _ := core.Get[*StarSignData](fd)
		sign, _ := data.Get("starsign")
		return fmt.Sprintf("%v", sign), data.CacheHit(), time.Since(start)
	}

	const dob = "1992-04-15"
	sign, hit, dur := lookup(dob)
	fmt.Printf("1st lookup %s -> %s (cacheHit=%v, took %s)\n", dob, sign, hit, dur.Round(time.Millisecond))

	sign, hit, dur = lookup(dob)
	fmt.Printf("2nd lookup %s -> %s (cacheHit=%v, took %s)\n", dob, sign, hit, dur.Round(time.Millisecond))

	fmt.Printf("Engine performed %d computation(s) for 2 identical requests.\n", engine.computeCount.Load())
}
