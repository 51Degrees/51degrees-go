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

// Package ipi_cloud is the IP Intelligence and IP geolocation Cloud Aspect
// Engine for Go, an alternative to MaxMind and IPinfo. It reads the
// IP Intelligence section of the response a Cloud Request Engine gets from
// the 51Degrees cloud service, so a Pipeline gives IP Intelligence values
// from the cloud as ipi_onpremise gives them from a data file.
//
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/pipeline-elements/cloud-aspect-engine.md
package ipi_cloud

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/aspectengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// DataKey is the key of the engine's Element Data, and the name of its
// section in the cloud response.
const DataKey = "ip"

// Engine is the IP Intelligence Cloud Aspect Engine. It must follow a Cloud
// Request Engine in the Pipeline, and gives the IP Intelligence Properties
// the Resource Key includes.
type Engine struct {
	*aspectengine.BaseCloudAspectEngine
}

// Data is the Element Data the engine adds to the Flow Data. Each Property is
// held under its name in lower case. A weighted Property holds an
// engines.AspectPropertyValue[[]core.WeightedValue[string]], and any other
// Property an engines.AspectPropertyValue[string].
type Data struct {
	*engines.BaseAspectData
}

// New creates the engine. A nil logger leaves the base engine's default.
func New(logger *slog.Logger) *Engine {
	return &Engine{
		BaseCloudAspectEngine: aspectengine.NewBaseCloudAspectEngine(DataKey, logger),
	}
}

// Process reads the IP Intelligence section of the cloud response into Data.
func (e *Engine) Process(fd core.FlowData) error {
	aspect, err := aspectengine.ParseAspect(fd, DataKey)
	if err != nil {
		return err
	}
	data := &Data{BaseAspectData: engines.NewBaseAspectData()}
	properties, err := cloudProperties(fd)
	if err != nil || len(properties) == 0 {
		setInferredValues(data, aspect)
	}
	for _, p := range properties {
		name := strings.ToLower(p.Name)
		if strings.Contains(strings.ToLower(p.Type), "weighted") {
			data.Set(name, weightedValue(aspect, name))
		} else {
			data.Set(name, textValue(aspect, name))
		}
	}
	fd.GetOrAdd(DataKey, func(core.FlowData) core.ElementData { return data })
	return nil
}

// nullReasonSuffix ends the name of the member that gives the reason a
// Property has no value.
const nullReasonSuffix = "nullreason"

// setInferredValues sets every Property in the section with no Property
// metadata to go by, taking a list of values that each have a weighting as
// weighted and anything else as text. A reason member is read with the
// Property it explains, not set as a Property of its own.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/pipeline-elements/cloud-aspect-engine.md#start-up-activity
func setInferredValues(data *Data, aspect map[string]json.RawMessage) {
	for name, raw := range aspect {
		if explained, ok := strings.CutSuffix(name, nullReasonSuffix); ok {
			if _, ok := aspect[explained]; ok {
				continue
			}
		}
		if isWeighted(raw) {
			data.Set(strings.ToLower(name), weightedValue(aspect, name))
		} else {
			data.Set(strings.ToLower(name), textValue(aspect, name))
		}
	}
}

// isWeighted reports whether raw is a list of values that each have a
// weighting, the form the cloud gives a weighted Property in.
func isWeighted(raw json.RawMessage) bool {
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 {
		return false
	}
	for _, item := range items {
		if !hasMember(item, "rawweighting") {
			return false
		}
	}
	return true
}

// hasMember reports whether object has a member named name, ignoring case,
// as encoding/json matches the weighting when it reads a weighted value.
func hasMember(object map[string]json.RawMessage, name string) bool {
	for member := range object {
		if strings.EqualFold(member, name) {
			return true
		}
	}
	return false
}

// cloudProperties gives the IP Intelligence Properties the Resource Key
// includes, from the Cloud Request Engine before this engine.
func cloudProperties(fd core.FlowData) ([]requestengine.PropertyMeta, error) {
	cre, ok := fd.Pipeline().GetElement(requestengine.DataKey).(*requestengine.CloudRequestEngine)
	if !ok {
		return nil, core.NewConfigurationError("no CloudRequestEngine found in the pipeline", nil)
	}
	products, err := cre.PublicProperties()
	if err != nil {
		return nil, err
	}
	return products[DataKey].Properties, nil
}

// textValue gives a Property that is not weighted as text, being a JSON
// string as it is and any other JSON value as the response writes it.
func textValue(aspect map[string]json.RawMessage, name string) engines.AspectPropertyValue[string] {
	raw := aspectengine.PropertyValue[json.RawMessage](aspect, name)
	value, err := raw.Value()
	if err != nil {
		return engines.NoValue[string](raw.NoValueMessage())
	}
	return engines.NewAspectPropertyValue(jsonText(value))
}

// weightedValue gives a weighted Property as its values, each with the raw
// weighting the response gives it.
func weightedValue(aspect map[string]json.RawMessage, name string) engines.AspectPropertyValue[[]core.WeightedValue[string]] {
	raw := aspectengine.PropertyValue[[]core.WeightedValue[json.RawMessage]](aspect, name)
	items, err := raw.Value()
	if err != nil {
		return engines.NoValue[[]core.WeightedValue[string]](raw.NoValueMessage())
	}
	values := make([]core.WeightedValue[string], 0, len(items))
	for _, item := range items {
		values = append(values, core.NewWeightedValue(jsonText(item.Value()), item.RawWeighting()))
	}
	return engines.NewAspectPropertyValue(values)
}

// jsonText gives a JSON string as it is, and any other JSON value as it is
// written.
func jsonText(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return string(raw)
}
