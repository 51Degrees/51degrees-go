// Example 3: a Flow Element whose input is gathered by client-side JavaScript.
//
// When no date of birth is available, the element returns a JavaScript Property
// containing a snippet that the client executes to collect it (here, it reads a
// browser cookie). The generated client-side script (from the JavaScript
// Builder) posts the value back, and a subsequent request can populate the star
// sign. This example demonstrates the flow headlessly: it processes once without
// the date, prints the generated JavaScript and JSON, then processes again with
// the value supplied as if returned by the client.
//
// Run with: go run ./examples/3_clientside_evidence
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/required-examples.md
package main

import (
	"fmt"
	"log"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/elements/jsonbuilder"
	"github.com/51Degrees/pipeline-go/engines"
	"github.com/51Degrees/pipeline-go/examples/starsign"
	"github.com/51Degrees/pipeline-go/web"
)

const (
	evidenceDateOfBirth = "query.date-of-birth"
	dobJavaScript       = `var d = (document.cookie.match(/(?:^|; )51D_date-of-birth=([^;]*)/)||[])[1];` +
		`if (d) { document.cookie = "51D_date-of-birth=" + d; }`
)

// ClientSideStarSignElement determines a star sign, or asks the client for the
// date of birth via a JavaScript Property when it is not yet available.
type ClientSideStarSignElement struct {
	*core.BaseElement
}

// NewClientSideStarSignElement creates the element.
func NewClientSideStarSignElement() *ClientSideStarSignElement {
	e := &ClientSideStarSignElement{BaseElement: core.NewBaseElement(
		"starsign", core.NewWhitelistFilter(evidenceDateOfBirth), nil)}
	e.SetProperties([]core.PropertyMetadata{
		{Name: "starsign", Element: e, Category: "Astrology", Available: true,
			EvidenceProperties: []string{"starsign.dobjavascript"}},
		{Name: "dobjavascript", Element: e, Category: "Astrology", Available: true, DelayExecution: false},
	})
	return e
}

// Process either returns the star sign or a JavaScript snippet to gather the
// date of birth from the client.
func (e *ClientSideStarSignElement) Process(fd core.FlowData) error {
	data := fd.GetOrAdd(e.DataKey(), func(core.FlowData) core.ElementData { return core.NewBaseElementData() })
	raw, ok := fd.Evidence().Get(evidenceDateOfBirth)
	if !ok {
		// Ask the client to gather the value; star sign has no value yet. Using
		// an AspectPropertyValue lets the JSON Builder emit the nullreason for us.
		data.Set("dobjavascript", core.JavaScript(dobJavaScript))
		data.Set("starsign", engines.NoValue[string]("Awaiting date of birth from client-side JavaScript."))
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
	pipeline, err := web.AssemblePipeline(web.PipelineConfig{}, NewClientSideStarSignElement())
	if err != nil {
		log.Fatal(err)
	}
	defer pipeline.Close()

	fmt.Println("=== First request: no date of birth yet ===")
	fd := pipeline.CreateFlowData()
	fd.AddEvidence("header.host", "example.com")
	if err := fd.Process(); err != nil {
		log.Fatal(err)
	}
	jsonData, _ := fd.Get(jsonbuilder.DataKey)
	fmt.Println("JSON delivered to the client:")
	fmt.Println(jsonData.(*jsonbuilder.Data).JSON())

	fmt.Println("\n=== Second request: client supplied the date of birth ===")
	fd2 := pipeline.CreateFlowData()
	fd2.AddEvidence(evidenceDateOfBirth, "1992-04-15")
	if err := fd2.Process(); err != nil {
		log.Fatal(err)
	}
	data, _ := fd2.Get("starsign")
	sign, _ := data.Get("starsign")
	fmt.Printf("Star sign: %v\n", sign)
}
