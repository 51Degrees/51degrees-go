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

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/examples/pipeline/starsign"
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
