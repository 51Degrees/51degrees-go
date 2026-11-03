/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2025 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
 *
 * This Original Work is licensed under the European Union Public Licence (EUPL)
 * v.1.2 and is subject to its terms as set out below.
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

package translation

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/51Degrees/ip-intelligence-go/v4/ipi_interop"
	"github.com/51Degrees/pipeline-go/core"
)

// IpiProcessor is the subset of the on-premise IP Intelligence engine that the
// adapter needs. *ipi_onpremise.Engine satisfies it; tests can substitute a
// fake.
type IpiProcessor interface {
	Process(ipAddress string) (ipi_interop.Values, error)
}

// IpiEngineElement adapts the standalone on-premise IP Intelligence engine to a
// pipeline flow element. The engine's own Process(ipAddress) signature cannot
// satisfy FlowElement.Process(FlowData), so it is wrapped here. The element
// reads the client IP from the evidence, processes it, and stores the weighted
// country-code lists under IPIElementDataKey for the translation engines
// downstream.
type IpiEngineElement struct {
	*core.BaseElement
	engine IpiProcessor
}

// NewIpiEngineElement creates the adapter around the given engine.
func NewIpiEngineElement(engine IpiProcessor, logger *slog.Logger) *IpiEngineElement {
	e := &IpiEngineElement{engine: engine}
	e.BaseElement = core.NewBaseElement(
		IPIElementDataKey,
		core.NewWhitelistFilter(EvidenceQueryClientIP, EvidenceServerClientIP),
		logger,
	)
	e.SetProperties([]core.PropertyMetadata{
		{Name: PropertyCountryCodesGeographical, Element: e,
			Category: "IpIntelligence", Available: true},
		{Name: PropertyCountryCodesPopulation, Element: e,
			Category: "IpIntelligence", Available: true},
	})
	return e
}

// Process looks up the client IP and stores the weighted country-code lists. The
// lists are always set (empty when there is no IP or no data) so downstream
// elements can rely on the Properties being present.
func (e *IpiEngineElement) Process(fd core.FlowData) error {
	data := fd.GetOrAdd(IPIElementDataKey, func(core.FlowData) core.ElementData {
		return core.NewBaseElementData()
	})
	data.Set(PropertyCountryCodesGeographical, []core.WeightedValue[string]{})
	data.Set(PropertyCountryCodesPopulation, []core.WeightedValue[string]{})

	ip := clientIP(fd)
	if ip == "" {
		fd.AddError(fmt.Errorf("no client IP address was supplied"), e)
		return nil
	}
	values, err := e.engine.Process(ip)
	if err != nil {
		fd.AddError(fmt.Errorf("IP Intelligence processing failed: %w", err), e)
		return nil
	}
	data.Set(PropertyCountryCodesGeographical,
		weightedCodes(values[PropertyCountryCodesGeographical]))
	data.Set(PropertyCountryCodesPopulation,
		weightedCodes(values[PropertyCountryCodesPopulation]))
	return nil
}

// clientIP returns the client IP from the evidence, checking the query key
// before the server key.
func clientIP(fd core.FlowData) string {
	for _, key := range []string{EvidenceQueryClientIP, EvidenceServerClientIP} {
		if v, ok := fd.Evidence().Get(key); ok {
			if s := fmt.Sprintf("%v", v); s != "" {
				return s
			}
		}
	}
	return ""
}

// weightedCodes converts the interop weighted values (ISO code strings) into the
// pipeline weighted-value type. The interop layer reports a 0..1 confidence; the
// pipeline weighted value carries a 16-bit raw weighting (matching the .NET
// IWeightedValue), so the confidence is scaled to that range, which preserves
// the ordering the country engines rank on.
func weightedCodes(values []*ipi_interop.WeightedValue) []core.WeightedValue[string] {
	out := make([]core.WeightedValue[string], 0, len(values))
	for _, wv := range values {
		code, ok := wv.Value.(string)
		if !ok {
			continue
		}
		out = append(out, core.NewWeightedValue(code, weightToRaw(wv.Weight)))
	}
	return out
}

// weightToRaw maps a 0..1 confidence weight to the pipeline's 16-bit raw
// weighting, clamping out-of-range inputs.
func weightToRaw(weight float64) uint16 {
	switch {
	case weight <= 0:
		return 0
	case weight >= 1:
		return math.MaxUint16
	default:
		return uint16(math.Round(weight * float64(math.MaxUint16)))
	}
}
