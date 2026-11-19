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

package onpremise

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/cache"
)

// PipelineEngine is the on-premise Device Detection engine as an On-premise
// Aspect Engine, to add to a Pipeline. It reads the evidence it accepts from
// each Flow Data and adds the DeviceData ProcessData gives under DataKey.
// That DeviceData reads each Property from the native results only when it
// is requested, and holds them until it is released. The caller releases
// them at once with Close on the DeviceData, got with core.Get[*DeviceData],
// when it has finished with the Flow Data. A finalizer releases them once
// nothing holds the DeviceData, as a backstop. The engine takes no results
// cache, because a cached DeviceData would be read after its release. Its
// Property metadata lists the match metric Properties after the data
// file's. Closing the engine, or the Pipeline that holds it, stops the
// engine.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/data-model.md?plain=1#L10-L12
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md?plain=1#L69-L70
type PipelineEngine struct {
	*engines.BaseOnPremiseAspectEngine
	engine    *Engine
	closeOnce sync.Once
}

// NewPipelineEngine creates the engine with the options New takes, ready to
// add to a Pipeline.
func NewPipelineEngine(opts ...EngineOptions) (*PipelineEngine, error) {
	engine, err := New(opts...)
	if err != nil {
		return nil, err
	}
	return newPipelineEngine(engine, nil), nil
}

// newPipelineEngine makes engine an On-premise Aspect Engine that logs to
// logger, the default logger when it is nil.
func newPipelineEngine(engine *Engine, logger *slog.Logger) *PipelineEngine {
	p := &PipelineEngine{engine: engine}
	p.BaseOnPremiseAspectEngine = engines.NewBaseOnPremiseAspectEngine(
		DataKey,
		core.NewFuncFilter(p.accepts),
		logger,
		engine.GetDataFile(),
		p.refreshFromFile,
		nil,
	)
	p.SetProperties(p.propertyMetadata())
	return p
}

// Engine gives the engine the Pipeline engine runs, for the metadata and the
// data file updates it offers.
func (p *PipelineEngine) Engine() *Engine { return p.engine }

// Process detects the device from the evidence in the Flow Data that the
// engine accepts, and adds the results as DeviceData, which reads no value
// until one is requested.
func (p *PipelineEngine) Process(fd core.FlowData) error {
	return p.Execute(fd, func() (engines.AspectData, error) {
		data, err := p.engine.ProcessData(p.evidence(fd))
		if err != nil {
			return nil, err
		}
		runtime.SetFinalizer(data, (*DeviceData).Free)
		return data, nil
	})
}

// errCacheRefused is the error SetCache gives.
var errCacheRefused = core.NewConfigurationError("a results cache cannot be "+
	"added to the on-premise Device Detection engine, because its DeviceData "+
	"holds native results that are released when the caller has finished "+
	"with them, and a cached DeviceData would be read after that", nil)

// SetCache refuses a results cache and adds none, because the specification
// says "the system MUST NOT allow a cache to be added to Engines where the
// Element Data they produce requires cleanup beyond what normal garbage
// collection will achieve", and this engine's DeviceData does. A nil cache
// adds nothing and gives no error.
//
// Deprecated: this engine takes no results cache. SetCache is kept because
// the engine's base type offers it, and always refuses a cache.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md?plain=1#L83-L85
func (p *PipelineEngine) SetCache(c cache.Cache) error {
	if c == nil {
		return nil
	}
	return errCacheRefused
}

// Close stops the engine, freeing its data set. A second call does nothing.
func (p *PipelineEngine) Close() error {
	p.closeOnce.Do(p.engine.Stop)
	return nil
}

// refreshFromFile reloads the engine's data file, the only file it reads.
func (p *PipelineEngine) refreshFromFile(path string) error {
	if filepath.Clean(path) != filepath.Clean(p.engine.GetDataFile()) {
		return core.NewConfigurationError(fmt.Sprintf("the Device Detection "+
			"engine reads %s, so it cannot refresh from %s",
			p.engine.GetDataFile(), path), nil)
	}
	return p.engine.RefreshData()
}

// evidencePrefixes are the prefixes of the evidence the engine reads, with
// the native prefix of each.
var evidencePrefixes = map[string]dd.EvidencePrefix{
	"header": dd.HttpHeaderString,
	"query":  dd.HttpEvidenceQuery,
	"cookie": dd.HttpEvidenceCookie,
	"server": dd.HttpEvidenceServer,
}

// upperPrefix starts the name of each header the engine reads when it is
// configured to use upper prefixed headers, as the names in a CGI
// environment are. It reads the header given with or without it.
const upperPrefix = "http_"

// sameHeader reports whether name, a header name the engine reads, names the
// header field.
func sameHeader(name, field string) bool {
	if strings.EqualFold(name, field) {
		return true
	}
	if len(name) > len(upperPrefix) &&
		strings.EqualFold(name[:len(upperPrefix)], upperPrefix) {
		return strings.EqualFold(name[len(upperPrefix):], field)
	}
	return false
}

// accepts reports whether the data file in use reads the evidence key. A
// header it reads can also be given in the query string, which then takes
// precedence.
func (p *PipelineEngine) accepts(key string) bool {
	prefix, field := core.SplitEvidenceKey(strings.ToLower(key))
	native, ok := evidencePrefixes[prefix]
	if !ok {
		return false
	}
	for _, k := range p.engine.GetHttpHeaderKeys() {
		if !sameHeader(k.Key, field) {
			continue
		}
		if k.Prefix == native ||
			(k.Prefix == dd.HttpHeaderString && native == dd.HttpEvidenceQuery) {
			return true
		}
	}
	return false
}

// evidence gives the evidence in the Flow Data that the engine accepts.
func (p *PipelineEngine) evidence(fd core.FlowData) []Evidence {
	var list []Evidence
	for _, key := range fd.Evidence().Keys() {
		if !p.accepts(key) {
			continue
		}
		value, _ := fd.Evidence().Get(key)
		prefix, field := core.SplitEvidenceKey(strings.ToLower(key))
		text, ok := value.(string)
		if !ok {
			text = fmt.Sprint(value)
		}
		list = append(list, Evidence{
			Prefix: evidencePrefixes[prefix],
			Key:    field,
			Value:  text,
		})
	}
	return list
}

// propertyMetadata gives the metadata of each Property the data file defines,
// then of each match metric Property, naming this engine as the element that
// populates it.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L448-L475
func (p *PipelineEngine) propertyMetadata() []core.PropertyMetadata {
	properties, err := p.engine.Properties()
	if err != nil {
		return nil
	}
	text, list := reflect.TypeOf(""), reflect.TypeOf([]string(nil))
	out := make([]core.PropertyMetadata, 0,
		len(properties)+len(metricProperties))
	for _, property := range properties {
		valueType := text
		if property.IsList {
			valueType = list
		}
		out = append(out, core.PropertyMetadata{
			Name:                  property.Name,
			Element:               p,
			Category:              property.Category,
			Type:                  valueType,
			Available:             property.Available,
			DataTiersWherePresent: property.DataTiers,
			Description:           property.Description,
		})
	}
	for _, metric := range metricProperties {
		valueType := text
		if metric.list {
			valueType = list
		}
		out = append(out, core.PropertyMetadata{
			Name:                  metric.name,
			Element:               p,
			Category:              matchMetricCategory,
			Type:                  valueType,
			Available:             true,
			DataTiersWherePresent: matchMetricTiers,
			Description:           metric.description,
		})
	}
	return out
}
