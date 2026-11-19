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
	"log/slog"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/cache"
)

// noAddressReason is the reason each Property has no value when the Flow
// Data holds no IP address the engine reads.
const noAddressReason = "No IP address was supplied in the evidence the engine reads."

// PipelineEngine is the on-premise IP Intelligence engine as an On-premise
// Aspect Engine, to add to a Pipeline in place of the IP Intelligence Cloud
// Engine. It passes the evidence it accepts to the native code, which
// chooses the IP address, and adds the Data under DataKey. That Data reads
// each Property from the native results only when it is requested, and holds
// them until it is released. Closing the Flow Data
// releases them at once, as the web middleware does when a request ends, and
// so does Close on the Data. A finalizer releases them once nothing holds the
// Data, as a backstop. The engine takes no results cache, because a
// cached Data would be read after its release. Closing the engine, or the
// Pipeline that holds it, stops the engine.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/cloud-aspect-engine.md?plain=1#L24-L26
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/resource-cleanup.md?plain=1#L60-L61
type PipelineEngine struct {
	*engines.BaseOnPremiseAspectEngine
	engine    *Engine
	closeOnce sync.Once
}

// NewPipelineEngine creates the engine with the options New takes, ready to
// add to a Pipeline. Concurrency is the processor count unless
// WithConcurrency or WithConfigIpi sets it.
func NewPipelineEngine(opts ...EngineOptions) (*PipelineEngine, error) {
	engine, err := New(append([]EngineOptions{withDefaultConcurrency()},
		opts...)...)
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

// Process passes the values of the evidence keys the data file lists to the
// native code, which chooses the IP address from them, and adds the results
// as a Data, which reads no value until one is requested. With none of those
// keys in the evidence, every Property has no value, and the message says
// why.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L114-L121
func (p *PipelineEngine) Process(fd core.FlowData) error {
	return p.Execute(fd, func() (engines.AspectData, error) {
		evidence, err := p.nativeEvidence(fd)
		if err != nil {
			return nil, err
		}
		if evidence == nil {
			return p.engine.noAddressData(), nil
		}
		defer evidence.Free()
		data, err := p.engine.processEvidence(evidence)
		if err != nil {
			return nil, err
		}
		runtime.SetFinalizer(data, (*Data).Free)
		return data, nil
	})
}

// errCacheRefused is the error SetCache gives.
var errCacheRefused = core.NewConfigurationError("a results cache cannot be "+
	"added to the on-premise IP Intelligence engine, because its Data holds "+
	"native results that are released when the caller has finished with "+
	"them, and a cached Data would be read after that", nil)

// SetCache refuses a results cache and adds none, because the specification
// says "the system MUST NOT allow a cache to be added to Engines where the
// Element Data they produce requires cleanup beyond what normal garbage
// collection will achieve", and this engine's Data does. A nil cache adds
// nothing and gives no error.
//
// Deprecated: this engine takes no results cache. SetCache is kept because
// the engine's base type offers it, and always refuses a cache.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/resource-cleanup.md?plain=1#L74-L76
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
		return core.NewConfigurationError(fmt.Sprintf("the IP Intelligence "+
			"engine reads %s, so it cannot refresh from %s",
			p.engine.GetDataFile(), path), nil)
	}
	return p.engine.RefreshData()
}

// evidencePrefixes are the prefixes of the evidence the engine reads, with
// the native prefix of each.
var evidencePrefixes = map[string]ipi_interop.EvidencePrefix{
	"query":  ipi_interop.HttpEvidenceQuery,
	"server": ipi_interop.HttpEvidenceServer,
}

// accepts reports whether the data file in use reads the evidence key.
func (p *PipelineEngine) accepts(key string) bool {
	prefix, field := core.SplitEvidenceKey(strings.ToLower(key))
	native, ok := evidencePrefixes[prefix]
	if !ok {
		return false
	}
	for _, k := range p.engine.GetHttpHeaderKeys() {
		if k.Prefix == native && strings.EqualFold(k.Key, field) {
			return true
		}
	}
	return false
}

// nativeEvidence gives the values of the evidence keys the data file lists,
// as native evidence for the native code to choose the address from, or nil
// when the Flow Data holds none of them with a value. The caller frees it.
func (p *PipelineEngine) nativeEvidence(fd core.FlowData) (*ipi_interop.Evidence, error) {
	names := map[ipi_interop.EvidencePrefix]string{}
	for name, native := range evidencePrefixes {
		names[native] = name
	}
	keys := p.engine.GetHttpHeaderKeys()
	var evidence *ipi_interop.Evidence
	for _, k := range keys {
		value, ok := fd.Evidence().Get(names[k.Prefix] + "." + strings.ToLower(k.Key))
		if !ok || value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok {
			text = fmt.Sprint(value)
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		if evidence == nil {
			evidence = ipi_interop.NewEvidenceWithCapacity(len(keys))
		}
		if err := evidence.Add(k.Prefix, k.Key, text); err != nil {
			evidence.Free()
			return nil, err
		}
	}
	return evidence, nil
}

// propertyMetadata gives the metadata of each Property the data file defines,
// naming this engine as the element that populates it.
func (p *PipelineEngine) propertyMetadata() []core.PropertyMetadata {
	properties, err := p.engine.Properties()
	if err != nil {
		return nil
	}
	text := reflect.TypeOf("")
	weighted := reflect.TypeOf([]core.WeightedValue[string](nil))
	out := make([]core.PropertyMetadata, 0, len(properties))
	for _, property := range properties {
		valueType := text
		if property.Type.IsWeighted() {
			valueType = weighted
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
	return out
}
