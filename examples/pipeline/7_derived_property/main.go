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

// Example 7: the Derived Property Element.
//
// The Derived Property Element computes new Properties from Properties that
// other elements in the Pipeline have already produced, following a script
// written in YAML or JSON. It holds no data file, sends nothing over the
// network and needs no Resource Key, so this example runs offline.
//
// The example builds the same Pipeline twice.
//
//  1. From config.json, which names each element's builder and passes the
//     script file ReaderEngagement.yaml through the ScriptFiles build
//     parameter.
//  2. In code, adding ReaderEngagement.yaml as a file and a second script
//     held as text in this file.
//
// Each Pipeline processes three requests. The third leaves out one source
// Property, and the derived Properties then have no value and a message
// naming the Property that was missing.
//
// The source Properties come from a small session element written here, so
// that nothing outside is needed. A real Pipeline puts Device Detection or
// IP Intelligence in that position instead.
//
// Run from examples/pipeline with: go run ./7_derived_property
//
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/required-examples.md#derived-property-element-example
package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"

	"github.com/51Degrees/51degrees-go/examples/pipeline/findoutmore"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/derivedproperty"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// The files beside this one. Both are read relative to the working
// directory, which main moves to this folder.
const (
	scriptFile = "ReaderEngagement.yaml"
	configFile = "config.json"
)

// The Properties the two scripts write.
const (
	engagementProperty = "ReaderEngagement"
	everyTestProperty  = "ReaderEngagementIsHigh"
)

// everyTestScript is a second script, held as text rather than in a file,
// to show that a script can come straight from code. It reads the same three
// source Properties and gives whether every test passed. JSON is read as
// well as YAML.
const everyTestScript = `
Format: 1
Name: ReaderEngagementIsHigh
Version: "1.0.0"
Output:
  Name: ReaderEngagementIsHigh
  Description: Whether every one of the ReaderEngagement tests passed on this request.
  ValueType: bool
  IsList: false
  Category: Example
Checks:
  SeveralPages: { Property: session.PagesViewed, Ge: 3 }
  LongEnough:   { Property: session.SecondsSincePageLoad, Ge: 30 }
  PointerMoved: { Property: session.PointerMoved, Eq: true }
Rules:
  - When: { Failed: Checks, Eq: 0 }
    Then: true
  - Else: false
`

// sessionKey is the session element's Element Data key, the part before the
// dot in each source Property a script names.
const sessionKey = "session"

// The Evidence the session element reads, as a page's own JavaScript might
// send it back in the query string.
const (
	evidencePagesViewed  = "query.pagesviewed"
	evidenceSeconds      = "query.secondssincepageload"
	evidencePointerMoved = "query.pointermoved"
)

// The session element's Properties, each set from the Evidence of the same
// position in sessionEvidence.
var (
	sessionProperties = []string{
		"PagesViewed", "SecondsSincePageLoad", "PointerMoved"}
	sessionEvidence = []string{
		evidencePagesViewed, evidenceSeconds, evidencePointerMoved}
	sessionTypes = []reflect.Type{
		reflect.TypeOf(0), reflect.TypeOf(0), reflect.TypeOf(false)}
)

// request is the Evidence for one request. A nil PointerMoved leaves that
// Evidence out.
type request struct {
	title        string
	pagesViewed  int
	seconds      int
	pointerMoved *bool
}

var (
	moved    = true
	requests = []request{
		{"Request 1, every test passes.", 5, 120, &moved},
		{"Request 2, the visitor left after 10 seconds.", 5, 10, &moved},
		{"Request 3, session.PointerMoved absent.", 5, 120, nil},
	}
)

func main() {
	moveToExampleFolder()
	runFromConfiguration()
	runFromCode()
	fmt.Print(findoutmore.Section("examples-pipeline-7_derived_property-main.go"))
}

// moveToExampleFolder makes this file's folder the working directory, so
// that the script and configuration file beside it are found whichever
// folder the example is started from. Go records the folder when it builds
// the program.
func moveToExampleFolder() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	folder := filepath.Dir(file)
	if _, err := os.Stat(filepath.Join(folder, scriptFile)); err != nil {
		return
	}
	if err := os.Chdir(folder); err != nil {
		log.Fatal(err)
	}
}

// runFromConfiguration builds the Pipeline from config.json, which names
// the builder of each element and passes the script file to the Derived
// Property Element through its ScriptFiles build parameter.
func runFromConfiguration() {
	fmt.Println("==========================================")
	fmt.Println("1. Built from " + configFile)
	fmt.Println()

	options, err := core.LoadPipelineOptions(configFile)
	if err != nil {
		log.Fatal(err)
	}
	// The registry finds each element's builder by the name the
	// configuration file gives.
	registry := core.NewBuilderRegistry()
	registry.Register("SessionElement", newSessionElement)
	registry.Register(derivedproperty.BuilderName,
		derivedproperty.NewFromConfiguration)
	pipeline, err := registry.BuildFromConfiguration(options, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer pipeline.Close()

	element := pipeline.GetElement(derivedproperty.DataKey).(*derivedproperty.Element)
	for _, script := range element.Scripts() {
		fmt.Printf("Script %s %s from %s\n",
			script.Name, script.Version, script.Source)
	}
	fmt.Println()
	for _, r := range requests {
		process(pipeline, r, engagementProperty)
	}
	fmt.Println("Request 3 gives no value at all, because a Property the " +
		"script names")
	fmt.Println("was not there. The message says which Property was " +
		"missing and why.")
	fmt.Println()
	printDefinitions(element)
}

// runFromCode builds the same Pipeline with the builders in code. One
// element runs both scripts, one read from a file and one held as text.
func runFromCode() {
	fmt.Println("==========================================")
	fmt.Println("2. Built in code")
	fmt.Println()

	session, err := newSessionElement(nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	derived, err := derivedproperty.NewBuilder().
		// A path may hold a wildcard, so "scripts/*.yaml" adds a folder.
		AddScriptFile(scriptFile).
		// The name given here must equal the Name the script carries.
		AddScript(everyTestProperty, everyTestScript).
		// AddBuiltInScript adds a script shipped in the package, from the
		// derived-properties repository. This example uses none, so that
		// it does not depend on what that repository holds.
		Build()
	if err != nil {
		log.Fatal(err)
	}
	// The Derived Property Element comes after every element that supplies
	// a source Property, or the build fails naming the Property.
	pipeline, err := core.NewPipelineBuilder().
		AddFlowElement(session).
		AddFlowElement(derived).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	defer pipeline.Close()

	for _, r := range requests {
		process(pipeline, r, engagementProperty, everyTestProperty)
	}
	fmt.Println("Both scripts read the same source Properties, so both " +
		"have no value")
	fmt.Println("for request 3.")
	fmt.Println()
}

// process runs one request and prints the source Properties and the
// derived Properties named.
func process(pipeline core.Pipeline, r request, names ...string) {
	fmt.Println(r.title)
	fd := pipeline.CreateFlowData()
	fd.AddEvidence(evidencePagesViewed, r.pagesViewed)
	fd.AddEvidence(evidenceSeconds, r.seconds)
	if r.pointerMoved != nil {
		fd.AddEvidence(evidencePointerMoved, *r.pointerMoved)
	}
	if err := fd.Process(); err != nil {
		log.Fatal(err)
	}

	// The source Properties, read as any element's Properties are read.
	session, err := fd.Get(sessionKey)
	if err != nil {
		log.Fatal(err)
	}
	for _, name := range sessionProperties {
		value, err := session.Get(name)
		if err != nil {
			fmt.Printf("  session.%s: not supplied on this request\n", name)
			continue
		}
		fmt.Printf("  session.%s: %v\n", name, value)
	}

	// The derived Properties, from the Element Data the Derived Property
	// Element writes under its key. Each value says whether it has a value
	// and, when it has none, why.
	stored, err := fd.Get(derivedproperty.DataKey)
	if err != nil {
		log.Fatal(err)
	}
	derived := stored.(*derivedproperty.Data)
	for _, name := range names {
		switch name {
		case everyTestProperty:
			printValue[bool](derived, name)
		default:
			printValue[string](derived, name)
		}
	}
	fmt.Println()
}

// printValue prints a derived Property whose script gives values of type T.
func printValue[T any](derived *derivedproperty.Data, name string) {
	value, err := derivedproperty.GetAs[engines.AspectPropertyValue[T]](
		derived, name)
	if err != nil {
		log.Fatal(err)
	}
	if v, err := value.Value(); err == nil {
		fmt.Printf("  derived.%s: %v\n", name, v)
		return
	}
	fmt.Printf("  derived.%s: no value. %s\n", name, value.NoValueMessage())
}

// printDefinitions prints the Property metadata the element gives the
// Pipeline, and the whole definition each script carries.
func printDefinitions(element *derivedproperty.Element) {
	fmt.Println("The Property metadata the element gives the Pipeline.")
	for _, p := range element.Properties() {
		fmt.Printf("  %s, type %v, category %s, available %t\n",
			p.Name, p.Type, p.Category, p.Available)
	}
	fmt.Println()
	for _, script := range element.Scripts() {
		output := script.Output
		fmt.Printf("The definition the script '%s' carries.\n", script.Name)
		fmt.Printf("  Description: %s\n", output.Description)
		fmt.Printf("  ValueType: %s\n", output.ValueType)
		if output.DefaultValue != nil {
			fmt.Printf("  DefaultValue: %s\n", *output.DefaultValue)
		}
		if output.Category != nil {
			fmt.Printf("  Category: %s\n", *output.Category)
		}
		fmt.Printf("  Dependencies: %v\n", output.Dependencies)
		for _, value := range output.Values {
			description := ""
			if value.Description != nil {
				description = *value.Description
			}
			fmt.Printf("  Value %-7s %s\n", value.Name, description)
		}
		fmt.Println()
	}
}

// sessionElement supplies the source Properties the scripts read, from the
// Evidence of each request.
type sessionElement struct{ *core.BaseElement }

// newSessionElement builds the session element. It takes no build
// parameters, and has the signature a builder registry needs.
func newSessionElement(_ map[string]any, logger *slog.Logger) (
	core.FlowElement, error) {
	e := &sessionElement{BaseElement: core.NewBaseElement(sessionKey,
		core.NewWhitelistFilter(sessionEvidence...), logger)}
	properties := make([]core.PropertyMetadata, len(sessionProperties))
	for i, name := range sessionProperties {
		properties[i] = core.PropertyMetadata{Name: name, Element: e,
			Type: sessionTypes[i], Available: true}
	}
	e.SetProperties(properties)
	return e, nil
}

// Process sets each Property whose Evidence the request carries. A Property
// left unset is absent, and a script that names it gives no value.
func (e *sessionElement) Process(fd core.FlowData) error {
	data := fd.GetOrAdd(sessionKey, func(core.FlowData) core.ElementData {
		return core.NewBaseElementData()
	})
	for i, key := range sessionEvidence {
		if value, ok := fd.Evidence().Get(key); ok {
			data.Set(sessionProperties[i], value)
		}
	}
	return nil
}
