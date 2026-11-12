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

// Example 2: an On-Premise Aspect Engine backed by an external data source.
//
// This builds on example 1 by moving the star sign date ranges out of code and
// into a data file. The engine implements the data-update Refreshable interface
// so the Data Update Service can reload it when the file changes, and produces
// AspectPropertyValue results that support the "no value" state.
//
// Run with: go run ./examples/2_onpremise_engine
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/required-examples.md
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/dataupdate"
	"github.com/51Degrees/51degrees-go/examples/pipeline/starsign"
)

const evidenceDateOfBirth = "query.date-of-birth"

// fileRange is the on-disk representation of a star sign date range.
type fileRange struct {
	Name       string `json:"name"`
	StartMonth int    `json:"startMonth"`
	StartDay   int    `json:"startDay"`
	EndMonth   int    `json:"endMonth"`
	EndDay     int    `json:"endDay"`
}

// StarSignData is the Aspect Data produced by the engine.
type StarSignData struct {
	*engines.BaseAspectData
}

// OnPremiseStarSignEngine determines the star sign using ranges loaded from a
// data file. Its range data can be refreshed at runtime in a thread-safe way.
type OnPremiseStarSignEngine struct {
	*engines.BaseOnPremiseAspectEngine
	mu     sync.RWMutex
	ranges []starsign.Range
}

// NewOnPremiseStarSignEngine builds the engine and loads its initial data file.
func NewOnPremiseStarSignEngine(dataFilePath string) (*OnPremiseStarSignEngine, error) {
	e := &OnPremiseStarSignEngine{}
	e.BaseOnPremiseAspectEngine = engines.NewBaseOnPremiseAspectEngine(
		"starsign",
		core.NewWhitelistFilter(evidenceDateOfBirth),
		nil,
		dataFilePath,
		e.RefreshFromFile,  // refresh-from-file
		e.RefreshFromBytes, // refresh-from-bytes
	)
	e.SetDataSourceTier("file")
	e.SetProperties([]core.PropertyMetadata{
		{Name: "starsign", Element: e, Category: "Astrology", Available: true},
	})
	if err := e.RefreshFromFile(dataFilePath); err != nil {
		return nil, err
	}
	return e, nil
}

// RefreshFromFile reloads the date ranges from the data file.
func (e *OnPremiseStarSignEngine) RefreshFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return e.RefreshFromBytes(data)
}

// RefreshFromBytes reloads the date ranges from an in-memory buffer.
func (e *OnPremiseStarSignEngine) RefreshFromBytes(data []byte) error {
	var fileRanges []fileRange
	if err := json.Unmarshal(data, &fileRanges); err != nil {
		return err
	}
	ranges := make([]starsign.Range, len(fileRanges))
	for i, fr := range fileRanges {
		ranges[i] = starsign.Range{
			Name:       fr.Name,
			StartMonth: time.Month(fr.StartMonth),
			StartDay:   fr.StartDay,
			EndMonth:   time.Month(fr.EndMonth),
			EndDay:     fr.EndDay,
		}
	}
	e.mu.Lock()
	e.ranges = ranges
	e.mu.Unlock()
	return nil
}

// Process determines the star sign for the supplied date of birth.
func (e *OnPremiseStarSignEngine) Process(fd core.FlowData) error {
	return e.Execute(fd, func() (engines.AspectData, error) {
		data := &StarSignData{BaseAspectData: engines.NewBaseAspectData()}
		raw, ok := fd.Evidence().Get(evidenceDateOfBirth)
		if !ok {
			data.Set("starsign", engines.NoValue[string]("No date of birth was supplied."))
			return data, nil
		}
		dob, err := starsign.ParseDate(fmt.Sprintf("%v", raw))
		if err != nil {
			return nil, err
		}
		e.mu.RLock()
		ranges := e.ranges
		e.mu.RUnlock()
		data.Set("starsign", engines.NewAspectPropertyValue(starsign.SignForRanges(ranges, dob.Month(), dob.Day())))
		return data, nil
	})
}

func main() {
	// Write an example data file (in practice this is distributed/updated by the
	// Data Update Service).
	dir, err := os.MkdirTemp("", "starsign-data")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	dataFile := filepath.Join(dir, "starsigns.json")
	if err := os.WriteFile(dataFile, defaultData(), 0o644); err != nil {
		log.Fatal(err)
	}

	engine, err := NewOnPremiseStarSignEngine(dataFile)
	if err != nil {
		log.Fatal(err)
	}
	pipeline, err := core.NewPipelineBuilder().AddFlowElement(engine).Build()
	if err != nil {
		log.Fatal(err)
	}
	defer pipeline.Close()

	// Register the engine for data updates so it reloads when the file changes.
	// The defaults watch the data file and poll an update URL, and this engine
	// has no update URL, so nothing is downloaded.
	updates := dataupdate.New(nil, nil)
	defer updates.Close()
	cfg := dataupdate.DefaultConfig()
	cfg.Identifier = "starsign"
	cfg.DataFilePath = dataFile
	if err := updates.Register(engine, "StarSignEngine", cfg); err != nil {
		log.Fatal(err)
	}

	lookup := func(dob string) {
		fd := pipeline.CreateFlowData()
		fd.AddEvidence(evidenceDateOfBirth, dob)
		if err := fd.Process(); err != nil {
			log.Fatal(err)
		}
		data, _ := core.Get[*StarSignData](fd)
		apv, _ := data.Get("starsign")
		sign, _ := apv.(engines.AspectPropertyValue[string]).Value()
		fmt.Printf("%s -> %s\n", dob, sign)
	}

	fmt.Println("Using initial data file:")
	lookup("1992-04-15")

	// Programmatically refresh from a modified data file.
	if err := os.WriteFile(dataFile, customData(), 0o644); err != nil {
		log.Fatal(err)
	}
	if err := updates.CheckForUpdate("starsign"); err != nil {
		log.Fatal(err)
	}
	fmt.Println("After refreshing with a custom data file (renames Aries to 'The Ram'):")
	lookup("1992-04-15")
}

func defaultData() []byte {
	ranges := make([]fileRange, len(starsign.DefaultRanges))
	for i, r := range starsign.DefaultRanges {
		ranges[i] = fileRange{r.Name, int(r.StartMonth), r.StartDay, int(r.EndMonth), r.EndDay}
	}
	out, _ := json.Marshal(ranges)
	return out
}

func customData() []byte {
	ranges := make([]fileRange, len(starsign.DefaultRanges))
	for i, r := range starsign.DefaultRanges {
		name := r.Name
		if name == "Aries" {
			name = "The Ram"
		}
		ranges[i] = fileRange{name, int(r.StartMonth), r.StartDay, int(r.EndMonth), r.EndDay}
	}
	out, _ := json.Marshal(ranges)
	return out
}
