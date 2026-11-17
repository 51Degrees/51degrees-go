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
)

// noAddressReason is the reason each Property has no value when the Flow
// Data holds no IP address the engine reads.
const noAddressReason = "No IP address was supplied in the evidence the engine reads."

// PipelineEngine is the on-premise IP Intelligence engine as an On-premise
// Aspect Engine, to add to a Pipeline in place of the IP Intelligence Cloud
// Engine. It looks up the first IP address in the evidence it accepts, in the
// order the data file gives, and adds the Data ProcessData gives under
// DataKey, with the caching and lazy loading the Pipeline's engines share.
// That Data reads each Property from the native results only when it is
// requested. A Flow Data has no Close, so the native results are freed when
// nothing holds the Data. Closing the engine, or the Pipeline that holds it,
// stops the engine.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/cloud-aspect-engine.md?plain=1#L24-L26
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

// Process looks up the IP address in the Flow Data's evidence, and adds the
// results as a Data, which reads no value until one is requested.
func (p *PipelineEngine) Process(fd core.FlowData) error {
	return p.Execute(fd, func() (engines.AspectData, error) {
		address, ok := p.address(fd)
		if !ok {
			return p.engine.noAddressData(), nil
		}
		data, err := p.engine.ProcessData(address)
		if err != nil {
			return nil, err
		}
		runtime.SetFinalizer(data, (*Data).Free)
		return data, nil
	})
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

// address gives the first IP address in the Flow Data's evidence, trying the
// keys in the order the data file gives them, every query key before any
// server key.
func (p *PipelineEngine) address(fd core.FlowData) (string, bool) {
	names := map[ipi_interop.EvidencePrefix]string{}
	for name, native := range evidencePrefixes {
		names[native] = name
	}
	for _, k := range p.engine.GetHttpHeaderKeys() {
		value, ok := fd.Evidence().Get(names[k.Prefix] + "." + strings.ToLower(k.Key))
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			text = fmt.Sprint(value)
		}
		if text = strings.TrimSpace(text); text != "" {
			return text, true
		}
	}
	return "", false
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
