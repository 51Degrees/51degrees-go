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

// Package humanconfidence is the Human Confidence Engine. It gives the two
// Properties a consumer reads, HumanConfidence and HumanConfidenceSource, in
// the derived Element Data, and is how a Pipeline redeems a sealed Human
// Confidence result that a page sends back to the publisher's server.
//
// A sealed result arrives as the Evidence query.humanconfidence, which the
// web integration adds from the page's humanconfidence parameter. The Cloud
// Request Engine sends it in its one request for the Flow Data, and the
// answer's derived member gives the redeemed Value. This engine reads it from
// there, so redeeming makes no request of its own and uses no other
// endpoint. A consumer never redeems a sealed result itself.
//
// The sealed result is bound to its page by a transaction. A server gives
// each page a requestengine.HumanConfidencePage, whose script address
// carries the page's transaction and whose ID the page sends back as
// humanconfidencepage. The Cloud Request Engine redeems the sealed result
// with the transaction it kept for that page, once. A request from a page
// with no kept transaction gives Medium.
//
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md
package humanconfidence

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/derivedproperty"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/missingproperty"
)

// The names of the two Properties, in the derived Element Data.
const (
	HumanConfidence       = "HumanConfidence"
	HumanConfidenceSource = "HumanConfidenceSource"
)

// Active is the HumanConfidenceSource of every request that carries a sealed
// result, redeemed or not, because the cloud service seals only an Active
// Value.
const Active = "Active"

// NotRedeemed is the HumanConfidence of a request whose sealed result was not
// redeemed. Human Confidence can't be worked out from a sealed result that
// didn't redeem, so it is never High or Low.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#where-the-sealed-result-is-not-redeemed
const NotRedeemed = "Medium"

// Engine is the Human Confidence Engine. Add it after the Cloud Request
// Engine, and after any Derived Property Element, whose Element Data it
// shares.
type Engine struct {
	*core.BaseElement
}

// New creates the engine. A nil logger leaves the default.
func New(logger *slog.Logger) *Engine {
	e := &Engine{BaseElement: core.NewBaseElement(derivedproperty.DataKey,
		core.NewWhitelistFilter(requestengine.HumanConfidenceEvidence,
			requestengine.HumanConfidencePageEvidence), logger)}
	text := reflect.TypeOf(engines.AspectPropertyValue[string]{})
	e.SetProperties([]core.PropertyMetadata{
		{Name: HumanConfidence, Element: e, Type: text, Available: true,
			Description: "How confident the cloud service is that a person is using the browser: High, Medium or Low."},
		{Name: HumanConfidenceSource, Element: e, Type: text, Available: true,
			Description: "Passive where HumanConfidence was worked out from the request alone, and Active where the browser tests ran."},
	})
	return e
}

// Factory builds the engine from configuration for core.BuilderRegistry. The
// engine has no options, so it takes no build parameters.
func Factory(_ map[string]any, logger *slog.Logger) (core.FlowElement, error) {
	return New(logger), nil
}

// OnPipelineAdded refuses a Pipeline whose Cloud Request Engine runs after
// this engine, because the engine reads that engine's answer. A Pipeline with
// no Cloud Request Engine is allowed, and gives Medium for every sealed
// result.
func (e *Engine) OnPipelineAdded(p core.Pipeline) error {
	if requestEngineAfter(p.FlowElements(), e) {
		return core.NewConfigurationError("the Human Confidence Engine needs "+
			"the Cloud Request Engine added to the Pipeline before it", nil)
	}
	return e.BaseElement.OnPipelineAdded(p)
}

// requestEngineAfter reports whether a Cloud Request Engine comes after e
// among elements, or in the parallel group that holds e.
func requestEngineAfter(elements []core.FlowElement, e *Engine) bool {
	seen := false
	for _, el := range elements {
		if holds(el, e) {
			seen = true
			if group, ok := el.(interface{ Children() []core.FlowElement }); ok {
				for _, child := range group.Children() {
					if _, ok := child.(*requestengine.CloudRequestEngine); ok {
						return true
					}
				}
			}
			continue
		}
		if _, ok := el.(*requestengine.CloudRequestEngine); ok && seen {
			return true
		}
	}
	return false
}

// holds reports whether el is e, or a group that holds e.
func holds(el core.FlowElement, e *Engine) bool {
	if el == core.FlowElement(e) {
		return true
	}
	if group, ok := el.(interface{ Children() []core.FlowElement }); ok {
		for _, child := range group.Children() {
			if holds(child, e) {
				return true
			}
		}
	}
	return false
}

// Process gives the two Properties. On a request that carries a sealed
// result it gives HumanConfidenceSource Active and HumanConfidence the
// redeemed Value, or Medium where the result was not redeemed, in place of
// any value another element gave. On any other request it gives each of the
// two that the answer's derived member holds, with the reason where it is
// null, unless a Derived Property Element has given it already. Where the
// answer leaves one out, gives HumanConfidence as anything but High, Medium,
// Low or Unknown, or the request failed, it gives no value with the reason
// Missing Properties gives. It returns no error of its own where a
// sealed result was not redeemed, so a read of either Property never fails
// for that reason. A failed request is still recorded once as the Cloud
// Request Engine's error.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#when-humanconfidence-arrives-sealed
// and https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#what-a-consumer-reads
func (e *Engine) Process(fd core.FlowData) error {
	if requestengine.CarriesSealedResult(fd) {
		return set(fd, map[string]engines.AspectPropertyValue[string]{
			HumanConfidence:       engines.NewAspectPropertyValue(redeemed(fd)),
			HumanConfidenceSource: engines.NewAspectPropertyValue(Active),
		}, true)
	}
	data, err := fd.Get(requestengine.DataKey)
	if err != nil {
		// With no Cloud Request Engine, or one that made no request, there is
		// no answer, so each Property has no value, with the reason.
		return set(fd, noAnswer(fd), false)
	}
	crd, ok := data.(*requestengine.CloudRequestData)
	if !ok {
		return set(fd, noAnswer(fd), false)
	}
	values := map[string]engines.AspectPropertyValue[string]{}
	derived, answered := answerDerived(fd)
	for _, name := range []string{HumanConfidence, HumanConfidenceSource} {
		value, given := propertyValue(derived, strings.ToLower(name))
		if given && name == HumanConfidence && !isLevel(derived[strings.ToLower(name)]) {
			// Text that is not one of the Property's values, such as a sealed
			// result, is not a level, so the Property is missing.
			given = false
		}
		if given {
			values[name] = value
			continue
		}
		reason := missingproperty.ForPropertyNotInResourceKey(name,
			derivedproperty.DataKey, derivedproperty.DataKey, nil)
		if !answered && crd.Failed() {
			reason = missingproperty.ForCloudRequestFailed(name, derivedproperty.DataKey)
		}
		values[name] = engines.NoValue[string](reason.Error())
	}
	return set(fd, values, false)
}

// NoCloudRequestEngine is the reason a Property has no value on a request
// with no sealed result, where the Pipeline has no Cloud Request Engine.
const NoCloudRequestEngine = "This property needs a Cloud Request Engine before the Human " +
	"Confidence Engine in the Pipeline, and there is none."

// noAnswer gives each of the two Properties no value, with the reason there
// is no answer: no Cloud Request Engine in the Pipeline, or one that made no
// request for fd, as in a recovery period.
func noAnswer(fd core.FlowData) map[string]engines.AspectPropertyValue[string] {
	values := map[string]engines.AspectPropertyValue[string]{}
	_, hasEngine := fd.Pipeline().GetElement(requestengine.DataKey).(*requestengine.CloudRequestEngine)
	for _, name := range []string{HumanConfidence, HumanConfidenceSource} {
		reason := NoCloudRequestEngine
		if hasEngine {
			reason = missingproperty.ForCloudRequestFailed(name, derivedproperty.DataKey).Error()
		}
		values[name] = engines.NoValue[string](reason)
	}
	return values
}

// set writes values into the derived Element Data, adding it where no
// element has. With replace false, a Property already given is kept.
func set(fd core.FlowData, values map[string]engines.AspectPropertyValue[string], replace bool) error {
	if len(values) == 0 {
		return nil
	}
	stored := fd.GetOrAdd(derivedproperty.DataKey,
		func(core.FlowData) core.ElementData { return derivedproperty.NewData() })
	data, ok := stored.(*derivedproperty.Data)
	if !ok {
		return fmt.Errorf("the element data under '%s' is %T, not the Derived "+
			"Property Element's", derivedproperty.DataKey, stored)
	}
	for name, value := range values {
		if !replace {
			if _, err := data.Get(name); err == nil {
				continue
			}
		}
		data.Set(name, value)
	}
	return nil
}

// redeemed gives the Value the answer redeemed the sealed result to, or
// Medium where there is no such Value: with no Cloud Request Engine or no
// License Key, with no transaction kept for the page, with no answer, as in a
// recovery period or after a failed request, or with an answer whose derived
// member gives anything other than High, Medium or Low.
func redeemed(fd core.FlowData) string {
	cre, ok := fd.Pipeline().GetElement(requestengine.DataKey).(*requestengine.CloudRequestEngine)
	if !ok || !cre.HasLicenseKey() {
		return NotRedeemed
	}
	data, err := fd.Get(requestengine.DataKey)
	if err != nil {
		return NotRedeemed
	}
	if crd, ok := data.(*requestengine.CloudRequestData); !ok || !crd.RedeemsHumanConfidence() {
		return NotRedeemed
	}
	derived, ok := answerDerived(fd)
	if !ok {
		return NotRedeemed
	}
	var value string
	if json.Unmarshal(derived["humanconfidence"], &value) != nil {
		return NotRedeemed
	}
	switch value {
	case "High", "Medium", "Low":
		return value
	}
	return NotRedeemed
}

// answerDerived gives the derived member of the Cloud Request Engine's answer
// for fd, with its keys in lower case, or false where there is no answer or
// no such member. It waits for the request.
func answerDerived(fd core.FlowData) (map[string]json.RawMessage, bool) {
	data, err := fd.Get(requestengine.DataKey)
	if err != nil {
		return nil, false
	}
	crd, ok := data.(*requestengine.CloudRequestData)
	if !ok {
		return nil, false
	}
	body, err := crd.Response()
	if err != nil {
		return nil, false
	}
	answer, ok := lowerKeys([]byte(body))
	if !ok {
		return nil, false
	}
	return lowerKeys(answer["derived"])
}

// propertyValue gives the Property name of derived, being its text, or no
// value with the reason the answer gives where it is null. It is false where
// derived does not hold the Property.
func propertyValue(derived map[string]json.RawMessage, name string) (engines.AspectPropertyValue[string], bool) {
	raw, ok := derived[name]
	if !ok {
		return engines.AspectPropertyValue[string]{}, false
	}
	var value string
	if string(raw) != "null" && json.Unmarshal(raw, &value) == nil {
		return engines.NewAspectPropertyValue(value), true
	}
	reason := "This property has no value in the cloud response."
	var given string
	if json.Unmarshal(derived[name+"nullreason"], &given) == nil && given != "" {
		reason = given
	}
	return engines.NoValue[string](reason), true
}

// levels are the values HumanConfidence can have: High, Medium, Low or
// Unknown.
var levels = []string{"High", "Medium", "Low", "Unknown"}

// isLevel reports whether raw, the answer's humanconfidence, is null or one of
// the values of HumanConfidence, exactly.
func isLevel(raw json.RawMessage) bool {
	if string(raw) == "null" {
		return true
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	for _, level := range levels {
		if value == level {
			return true
		}
	}
	return false
}

// lowerKeys parses raw as a JSON object, with its keys in lower case.
func lowerKeys(raw []byte) (map[string]json.RawMessage, bool) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, false
	}
	out := make(map[string]json.RawMessage, len(object))
	for key, value := range object {
		out[strings.ToLower(key)] = value
	}
	return out, true
}
