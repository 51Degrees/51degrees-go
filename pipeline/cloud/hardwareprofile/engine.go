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

// Package hardwareprofile is the Hardware Profile Lookup Cloud Engine. It
// reads the hardware section of the response a Cloud Request Engine gets from
// the 51Degrees cloud service for a profile lookup, such as the devices that
// match a TAC (query.tac) or a native model name (query.nativemodel), and
// gives each matching device's profile with its Properties.
//
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/hardware-profile-lookup-cloud.md
package hardwareprofile

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
const DataKey = "hardware"

// ProfilesProperty is the name of the engine's one top level Property, which
// holds the matching profiles.
const ProfilesProperty = "profiles"

// nullReasonSuffix ends the name of the member that gives the reason a
// Property has no value.
const nullReasonSuffix = "nullreason"

// Engine is the Hardware Profile Lookup Cloud Engine. It must follow a Cloud
// Request Engine in the Pipeline, and gives the Properties of each profile
// the cloud service returns for the lookup.
type Engine struct {
	*aspectengine.BaseCloudAspectEngine
}

// Data is the Element Data the engine adds to the Flow Data. Its one
// Property, profiles, holds an engines.AspectPropertyValue[[]*DeviceData].
type Data struct {
	*engines.BaseAspectData
}

// Profiles returns the profiles that matched the lookup, one DeviceData for
// each, in the order the cloud service gave them.
func (d *Data) Profiles() engines.AspectPropertyValue[[]*DeviceData] {
	v, err := d.Get(ProfilesProperty)
	if err != nil {
		return engines.NoValue[[]*DeviceData](err.Error())
	}
	profiles, ok := v.(engines.AspectPropertyValue[[]*DeviceData])
	if !ok {
		return engines.NoValue[[]*DeviceData]("profiles is not a list of profiles")
	}
	return profiles
}

// DeviceData is one profile that matched the lookup. Each of its Properties,
// the sub-properties of profiles, is held under its name in lower case and
// read with Get, as the Properties of the other cloud Aspect Data are. A
// Property whose value is a JSON list holds an
// engines.AspectPropertyValue[[]string], and any other Property an
// engines.AspectPropertyValue[string].
type DeviceData struct {
	*engines.BaseAspectData
}

// New creates the engine. A nil logger leaves the base engine's default.
func New(logger *slog.Logger) *Engine {
	return &Engine{
		BaseCloudAspectEngine: aspectengine.NewBaseCloudAspectEngine(DataKey, logger),
	}
}

// Factory builds the engine from configuration for core.BuilderRegistry, so
// a Pipeline holding it can be built from a configuration file. The engine
// has no configuration options, so it takes no build parameters.
func Factory(_ map[string]any, logger *slog.Logger) (core.FlowElement, error) {
	return New(logger), nil
}

// OnPipelineAdded initializes the engine from the Cloud Request Engine, which
// must have been added to the Pipeline before it. A Pipeline with no Cloud
// Request Engine before this engine is refused.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/hardware-profile-lookup-cloud.md?plain=1#L36-L38
func (e *Engine) OnPipelineAdded(p core.Pipeline) error {
	if !requestEngineBefore(p, e) {
		return core.NewConfigurationError("the Hardware Profile Lookup Cloud "+
			"Engine needs a Cloud Request Engine added to the Pipeline before it", nil)
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

// Process reads the hardware section of the cloud response into Data, with a
// DeviceData for each profile. Where the cloud request failed, the failure
// is already the Cloud Request Engine's error, so Process adds no Data and
// returns nil.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/cloud-aspect-engine.md?plain=1#L79-L85
func (e *Engine) Process(fd core.FlowData) error {
	aspect, err := aspectengine.ParseAspect(fd, DataKey)
	if err != nil {
		if e.CloudRequestFailed(fd) {
			return nil
		}
		return err
	}
	data := &Data{BaseAspectData: engines.NewBaseAspectData()}
	data.Set(ProfilesProperty, profilesValue(aspect, itemNames(e.Properties())))
	fd.GetOrAdd(DataKey, func(core.FlowData) core.ElementData { return data })
	return nil
}

// itemNames gives the names, in lower case, of the sub-properties of the
// profiles Property, from the Property metadata the Cloud Request Engine
// gave. It is empty when that metadata is not known.
func itemNames(properties []core.PropertyMetadata) []string {
	for _, p := range properties {
		if strings.EqualFold(p.Name, ProfilesProperty) {
			names := make([]string, 0, len(p.ItemProperties))
			for _, item := range p.ItemProperties {
				names = append(names, strings.ToLower(item.Name))
			}
			return names
		}
	}
	return nil
}

// profilesValue gives the profiles in the section, each with its
// sub-properties. Where the sub-properties are known from the metadata, every
// one of them is set on each profile, as no value with the cloud's reason
// where the profile has none. Where they are not known, each member of the
// profile is set, apart from a reason, which is read with the Property it
// explains.
func profilesValue(aspect map[string]json.RawMessage, items []string) engines.AspectPropertyValue[[]*DeviceData] {
	raw := aspectengine.PropertyValue[[]map[string]json.RawMessage](aspect, ProfilesProperty)
	objects, err := raw.Value()
	if err != nil {
		return engines.NoValue[[]*DeviceData](raw.NoValueMessage())
	}
	profiles := make([]*DeviceData, 0, len(objects))
	for _, object := range objects {
		profile := lowerKeys(object)
		device := &DeviceData{BaseAspectData: engines.NewBaseAspectData()}
		names := items
		if len(names) == 0 {
			names = memberNames(profile)
		}
		for _, name := range names {
			device.Set(name, subPropertyValue(profile, name))
		}
		profiles = append(profiles, device)
	}
	return engines.NewAspectPropertyValue(profiles)
}

// lowerKeys gives object with each member's name in lower case.
func lowerKeys(object map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(object))
	for name, value := range object {
		out[strings.ToLower(name)] = value
	}
	return out
}

// memberNames gives the names of the members of profile that are Properties,
// leaving out a reason member whose Property is also present.
func memberNames(profile map[string]json.RawMessage) []string {
	names := make([]string, 0, len(profile))
	for name := range profile {
		if explained, ok := strings.CutSuffix(name, nullReasonSuffix); ok {
			if _, ok := profile[explained]; ok {
				continue
			}
		}
		names = append(names, name)
	}
	return names
}

// subPropertyValue gives a sub-property of a profile, a JSON list as its
// items' text and any other JSON value as text. A sub-property that is absent
// or null has no value, with the reason the cloud gave.
func subPropertyValue(profile map[string]json.RawMessage, name string) any {
	raw, ok := profile[name]
	if ok && isList(raw) {
		listed := aspectengine.PropertyValue[[]json.RawMessage](profile, name)
		items, err := listed.Value()
		if err != nil {
			return engines.NoValue[[]string](listed.NoValueMessage())
		}
		values := make([]string, 0, len(items))
		for _, item := range items {
			values = append(values, jsonText(item))
		}
		return engines.NewAspectPropertyValue(values)
	}
	value := aspectengine.PropertyValue[json.RawMessage](profile, name)
	text, err := value.Value()
	if err != nil {
		return engines.NoValue[string](value.NoValueMessage())
	}
	return engines.NewAspectPropertyValue(jsonText(text))
}

// isList reports whether raw is a JSON list.
func isList(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return strings.HasPrefix(s, "[")
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
