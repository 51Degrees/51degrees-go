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

// Package dd_cloud is the Device Detection Cloud Aspect Engine. It reads the
// Device Detection section of the response a Cloud Request Engine gets from
// the 51Degrees cloud service, so a Pipeline gives Device Detection
// Properties from the cloud. It uses no C code, so it needs no cgo, and it
// is in the Pipeline module beside the shared Cloud Engines rather than in the
// Device Detection module, whose packages need cgo.
//
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/device-detection-cloud.md
package dd_cloud

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
const DataKey = "device"

// Engine is the Device Detection Cloud Aspect Engine. It must follow a Cloud
// Request Engine in the Pipeline, and gives the Device Detection Properties
// the Resource Key includes.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/device-detection-cloud.md?plain=1#L22-L24
type Engine struct {
	*aspectengine.BaseCloudAspectEngine
}

// Data is the Element Data the engine adds to the Flow Data. Each Property is
// held under its name in lower case. A Property the cloud gives as a list
// holds an engines.AspectPropertyValue[[]string], and any other Property an
// engines.AspectPropertyValue[string] with the text of its value, such as
// "true" for a Boolean.
type Data struct {
	*engines.BaseAspectData
}

// New creates the engine. A nil logger leaves the base engine's default.
func New(logger *slog.Logger) *Engine {
	return &Engine{
		BaseCloudAspectEngine: aspectengine.NewBaseCloudAspectEngine(DataKey, logger),
	}
}

// OnPipelineAdded initializes the engine from the Cloud Request Engine, which
// must have been added to the Pipeline before it. A Pipeline with no Cloud
// Request Engine before this engine is refused.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/device-detection-cloud.md?plain=1#L22-L24
func (e *Engine) OnPipelineAdded(p core.Pipeline) error {
	if !requestEngineBefore(p, e) {
		return core.NewConfigurationError("the Device Detection Cloud Engine "+
			"needs a Cloud Request Engine added to the Pipeline before it", nil)
	}
	return e.BaseCloudAspectEngine.OnPipelineAdded(p)
}

// requestEngineBefore reports whether p runs a Cloud Request Engine before e,
// being one of p's elements before the element that is e or the group run in
// parallel that holds e.
func requestEngineBefore(p core.Pipeline, e *Engine) bool {
	seen := false
	for _, el := range p.FlowElements() {
		if holds(el, e) {
			return seen
		}
		if _, ok := el.(*requestengine.CloudRequestEngine); ok {
			seen = true
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

// Factory builds the engine from configuration for core.BuilderRegistry, so
// a Pipeline holding it can be built from a configuration file. The engine
// has no options, so it takes no build parameters.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/device-detection-cloud.md?plain=1#L48-L50
func Factory(_ map[string]any, logger *slog.Logger) (core.FlowElement, error) {
	return New(logger), nil
}

// nullReasonSuffix ends the name of the member that gives the reason a
// Property has no value.
const nullReasonSuffix = "nullreason"

// Process reads the Device Detection section of the cloud response into
// Data. Parsing the response and the reason a Property has no value is the
// shared Cloud Aspect Engine logic's.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/device-detection-cloud.md?plain=1#L39-L46
func (e *Engine) Process(fd core.FlowData) error {
	aspect, err := aspectengine.ParseAspect(fd, DataKey)
	if err != nil {
		if e.CloudRequestFailed(fd) {
			return nil
		}
		return err
	}
	data := &Data{BaseAspectData: engines.NewBaseAspectData()}
	for name, raw := range aspect {
		if explained, ok := strings.CutSuffix(name, nullReasonSuffix); ok {
			if _, ok := aspect[explained]; ok {
				continue
			}
		}
		key := strings.ToLower(name)
		if isList(raw) {
			data.Set(key, aspectengine.PropertyValue[[]string](aspect, name))
		} else {
			data.Set(key, textValue(aspect, name))
		}
	}
	fd.GetOrAdd(DataKey, func(core.FlowData) core.ElementData { return data })
	return nil
}

// isList reports whether raw is a JSON list of strings. A JSON null is not a
// list, although encoding/json reads it into one.
func isList(raw json.RawMessage) bool {
	var items []string
	trimmed := strings.TrimSpace(string(raw))
	return strings.HasPrefix(trimmed, "[") && json.Unmarshal(raw, &items) == nil
}

// textValue gives a Property that is not a list as text, being a JSON string
// as it is and any other JSON value as the response writes it.
func textValue(aspect map[string]json.RawMessage, name string) engines.AspectPropertyValue[string] {
	raw := aspectengine.PropertyValue[json.RawMessage](aspect, name)
	value, err := raw.Value()
	if err != nil {
		return engines.NoValue[string](raw.NoValueMessage())
	}
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		return engines.NewAspectPropertyValue(text)
	}
	return engines.NewAspectPropertyValue(string(value))
}
