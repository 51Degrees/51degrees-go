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

package ipi_onpremise

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// DataKey is the key of the engine's Element Data in a Flow Data, the same as
// the IP Intelligence Cloud Engine's.
const DataKey = "ip"

// noValuesReason is the reason a Property has no value when the data file
// holds none for the address.
const noValuesReason = "The data file holds no value of this Property for the IP address."

// IpData is the Aspect Data the engine gives, from ProcessData or as a
// Pipeline engine, in the form the IP Intelligence Cloud Engine gives its
// Element Data in. Each Property is held under its name in lower case. A
// Property the data file weights gives an
// engines.AspectPropertyValue[[]core.WeightedValue[string]], each value with
// its raw weighting out of core.MaxRawWeighting, and any other Property an
// engines.AspectPropertyValue[string]. A Property with no value says why.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/cloud-aspect-engine.md?plain=1#L24-L26
type IpData struct {
	*engines.BaseAspectData
}

// ProcessData looks the IP address up, as Process does, and gives the results
// as Aspect Data.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/cloud-aspect-engine.md?plain=1#L24-L26
func (e *Engine) ProcessData(ipAddress string) (*IpData, error) {
	values, err := e.Process(ipAddress)
	if err != nil {
		return nil, err
	}
	return e.ipData(values, noValuesReason), nil
}

// ipData gives each Property the engine was made with its values, or no
// value with the reason given.
func (e *Engine) ipData(values ipi_interop.Values, reason string) *IpData {
	data := &IpData{BaseAspectData: engines.NewBaseAspectData()}
	weighted := e.weightedProperties()
	for i := 0; ; i++ {
		name := e.GetPropertyNameByIndex(i)
		if name == "" {
			break
		}
		key := strings.ToLower(name)
		items := values[name]
		switch {
		case weighted[key] && len(items) > 0:
			data.Set(key, engines.NewAspectPropertyValue(weightedValues(items)))
		case weighted[key]:
			data.Set(key, engines.NoValue[[]core.WeightedValue[string]](reason))
		case len(items) > 0:
			data.Set(key, engines.NewAspectPropertyValue(valueText(items[0].Value)))
		default:
			data.Set(key, engines.NoValue[string](reason))
		}
	}
	return data
}

// weightedValues gives each value as text, with its weighting as a raw
// weighting out of core.MaxRawWeighting, as the cloud gives it.
func weightedValues(items []*ipi_interop.WeightedValue) []core.WeightedValue[string] {
	out := make([]core.WeightedValue[string], 0, len(items))
	for _, item := range items {
		weight := math.Max(0, math.Min(1, item.Weight))
		raw := uint16(math.Round(weight * core.MaxRawWeighting))
		out = append(out, core.NewWeightedValue(valueText(item.Value), raw))
	}
	return out
}

// valueText gives a value as the text the cloud gives it as.
func valueText(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	default:
		return fmt.Sprint(v)
	}
}

// weightedProperties gives, by lower case name, the Properties the data file
// in use weights. The answer is kept once the data file has given it.
func (e *Engine) weightedProperties() map[string]bool {
	if weighted := e.weighted.Load(); weighted != nil {
		return *weighted
	}
	properties, err := e.Properties()
	if err != nil {
		return map[string]bool{}
	}
	weighted := make(map[string]bool, len(properties))
	for _, p := range properties {
		if p.Type.IsWeighted() {
			weighted[strings.ToLower(p.Name)] = true
		}
	}
	e.weighted.Store(&weighted)
	return weighted
}
