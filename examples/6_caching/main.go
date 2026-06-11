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

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/engines"
	"github.com/51Degrees/pipeline-go/engines/cache"
	"github.com/51Degrees/pipeline-go/examples/starsign"
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
