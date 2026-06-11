// Example 1: a simple Flow Element with hard-coded logic.
//
// This is the most basic example of creating a Flow Element. It reads a date of
// birth from Evidence and populates a "starsign" Property using hard-coded date
// ranges.
//
// Run with: go run ./examples/1_simple_element
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/required-examples.md
package main

import (
	"fmt"
	"log"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/examples/starsign"
)

// EvidenceDateOfBirth is the Evidence key the element reads.
const EvidenceDateOfBirth = "query.date-of-birth"

// StarSignElement is a simple Flow Element with hard-coded star sign logic.
type StarSignElement struct {
	*core.BaseElement
}

// NewStarSignElement creates the element.
func NewStarSignElement() *StarSignElement {
	e := &StarSignElement{BaseElement: core.NewBaseElement("starsign", core.NewWhitelistFilter(EvidenceDateOfBirth), nil)}
	e.SetProperties([]core.PropertyMetadata{
		{Name: "starsign", Element: e, Category: "Astrology", Available: true},
	})
	return e
}

// Process reads the date of birth from Evidence and writes the star sign.
func (e *StarSignElement) Process(fd core.FlowData) error {
	data := fd.GetOrAdd(e.DataKey(), func(core.FlowData) core.ElementData { return core.NewBaseElementData() })
	raw, ok := fd.Evidence().Get(EvidenceDateOfBirth)
	if !ok {
		data.Set("starsign", "Unknown")
		return nil
	}
	dob, err := starsign.ParseDate(fmt.Sprintf("%v", raw))
	if err != nil {
		return err
	}
	data.Set("starsign", starsign.SignForDate(dob.Month(), dob.Day()))
	return nil
}

func main() {
	pipeline, err := core.NewPipelineBuilder().AddFlowElement(NewStarSignElement()).Build()
	if err != nil {
		log.Fatal(err)
	}
	defer pipeline.Close()

	for _, dob := range []string{"1992-04-15", "1990-12-25", "2001-08-01"} {
		fd := pipeline.CreateFlowData()
		fd.AddEvidence(EvidenceDateOfBirth, dob)
		if err := fd.Process(); err != nil {
			log.Fatal(err)
		}
		data, _ := fd.Get("starsign")
		sign, _ := data.Get("starsign")
		fmt.Printf("%s -> %s\n", dob, sign)
	}
}
