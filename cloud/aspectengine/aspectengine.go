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

// Package aspectengine provides the base for Cloud Aspect Engines, which take
// the raw JSON produced by the Cloud Request Engine and deserialize the portion
// relevant to their Aspect into typed Element Data.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-aspect-engine.md
package aspectengine

import (
	"encoding/json"
	"log/slog"

	"github.com/51Degrees/pipeline-go/cloud/requestengine"
	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/engines"
)

// BaseCloudAspectEngine supplies the shared behavior for Cloud Aspect Engines.
// Concrete engines embed it, set their data key and properties, and implement
// Process using the parsing helpers provided here.
//
// Cloud Aspect Engines consume no Evidence directly and MUST NOT have a cache
// added (there is no Evidence key to cache on); apply caching to the Cloud
// Request Engine instead.
type BaseCloudAspectEngine struct {
	*engines.BaseAspectEngine
}

// NewBaseCloudAspectEngine creates a Cloud Aspect Engine base with the given
// data key. The evidence filter is empty because these engines use no Evidence.
func NewBaseCloudAspectEngine(dataKey string, logger *slog.Logger) *BaseCloudAspectEngine {
	e := &BaseCloudAspectEngine{
		BaseAspectEngine: engines.NewBaseAspectEngine(dataKey, core.NewWhitelistFilter(), logger),
	}
	e.SetDataSourceTier("cloud")
	return e
}

// ResponseJSON returns the raw JSON response produced by the Cloud Request
// Engine for this Flow Data.
func ResponseJSON(fd core.FlowData) (string, error) {
	data, err := fd.Get(requestengine.DataKey)
	if err != nil {
		return "", core.NewDataError("cloud response not found; a CloudRequestEngine must run before a CloudAspectEngine")
	}
	v, err := data.Get(requestengine.JSONResponseProperty)
	if err != nil {
		return "", err
	}
	s, ok := v.(string)
	if !ok {
		return "", core.NewDataError("cloud response is not a string")
	}
	return s, nil
}

// ParseAspect parses the cloud response and returns the JSON object for the
// given Aspect (for example "device"), as a map of raw JSON values keyed by
// Property name.
func ParseAspect(fd core.FlowData, aspectKey string) (map[string]json.RawMessage, error) {
	raw, err := ResponseJSON(fd)
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, requestengine.NewCloudRequestError("could not parse cloud JSON response", err)
	}
	section, ok := doc[aspectKey]
	if !ok {
		return nil, core.NewDataError("cloud response does not contain aspect '" + aspectKey + "'")
	}
	var aspect map[string]json.RawMessage
	if err := json.Unmarshal(section, &aspect); err != nil {
		return nil, requestengine.NewCloudRequestError("could not parse aspect '"+aspectKey+"' from cloud response", err)
	}
	return aspect, nil
}

// PropertyValue extracts a single Property from a parsed Aspect object as an
// AspectPropertyValue. If the JSON value is absent or null, the helper looks for
// a sibling Property suffixed with "nullreason" and uses it as the no-value
// message, implementing the Null Values feature.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-aspect-engine.md#processing
func PropertyValue[T any](aspect map[string]json.RawMessage, name string) engines.AspectPropertyValue[T] {
	raw, ok := aspect[name]
	if ok && !isJSONNull(raw) {
		var v T
		if err := json.Unmarshal(raw, &v); err == nil {
			return engines.NewAspectPropertyValue(v)
		}
	}
	msg := "This property has no value in the cloud response."
	if reasonRaw, ok := aspect[name+"nullreason"]; ok && !isJSONNull(reasonRaw) {
		var reason string
		if err := json.Unmarshal(reasonRaw, &reason); err == nil && reason != "" {
			msg = reason
		}
	}
	return engines.NoValue[T](msg)
}

func isJSONNull(raw json.RawMessage) bool {
	s := string(raw)
	return s == "" || s == "null"
}

// AspectProperties derives Property metadata for the given Aspect from the Cloud
// Request Engine that precedes this engine in the Pipeline. Per the
// specification this is treated as lazily initialized, failable data and is NOT
// cached, so it is re-derived (and the cloud metadata re-initialized if needed)
// on each call.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-aspect-engine.md#start-up-activity
func AspectProperties(fd core.FlowData, element core.FlowElement, aspectKey string) ([]core.PropertyMetadata, error) {
	re := fd.Pipeline().GetElement(requestengine.DataKey)
	cre, ok := re.(*requestengine.CloudRequestEngine)
	if !ok {
		return nil, core.NewConfigurationError("no CloudRequestEngine found in the pipeline", nil)
	}
	products, err := cre.PublicProperties()
	if err != nil {
		return nil, err
	}
	product, ok := products[aspectKey]
	if !ok {
		return nil, nil
	}
	out := make([]core.PropertyMetadata, 0, len(product.Properties))
	for _, p := range product.Properties {
		out = append(out, core.PropertyMetadata{
			Name:      p.Name,
			Element:   element,
			Category:  p.Category,
			Available: true,
		})
	}
	return out, nil
}
