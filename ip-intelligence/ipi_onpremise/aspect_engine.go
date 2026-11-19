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
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// DataKey is the key of the Element Data the AspectEngine adds to a Flow
// Data. It is the key the IP Intelligence Cloud Aspect Engine uses, so code
// that reads the Element Data by key works with either.
const DataKey = "ip"

// The prefixes of the Evidence keys the Engine accepts, as a Flow Data
// writes them.
const (
	queryPrefix  = "query"
	serverPrefix = "server"
)

// AspectEngine is the IP Intelligence On-premise Engine as a Flow Element,
// for use in a Pipeline. It takes the IP address from the Evidence of a Flow
// Data and adds an AspectData under DataKey, which reads each Property from
// the data file when the Property is requested.
//
// It creates and owns an Engine, which loads the data file and keeps it up to
// date. Close stops that Engine, and a Pipeline closes its elements when it
// is closed.
//
// The Engine takes no results cache, because its Element Data reads from
// native memory that is freed when the Element Data is closed.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md
type AspectEngine struct {
	*core.BaseElement
	engine *Engine

	// positions holds the position of each Property the Engine was created
	// with, by its name in lower case. It is made once, when the Engine is
	// created, and never from a later data file.
	positions map[string]int
	// names holds those Properties' names in lower case, by position.
	names []string
	// metadata is the Property metadata of the data file in use, and
	// accepted the Evidence keys it lists, in the order the native code
	// tries them. Both are worked out each time a data file is loaded.
	metadata atomic.Pointer[[]core.PropertyMetadata]
	accepted atomic.Pointer[[]acceptedKey]

	closeOnce sync.Once
}

// valueType is the type of the value AspectData gives for every Property.
var valueType = reflect.TypeOf(
	engines.AspectPropertyValue[[]core.WeightedValue[string]]{})

// NewAspectEngine creates the IP Intelligence On-premise Engine as a Flow
// Element. It takes the options New takes, and WithDataFile is required. A
// setting New refuses is refused here in the same way.
//
// Concurrency is the number of processors unless WithConcurrency or
// WithConfigIpi sets it.
func NewAspectEngine(opts ...EngineOptions) (*AspectEngine, error) {
	return newAspectEngine(nil, opts)
}

// newAspectEngine creates the AspectEngine with the logger its Flow Element
// logs to, which may be nil.
func newAspectEngine(logger *slog.Logger, opts []EngineOptions) (*AspectEngine, error) {
	a := &AspectEngine{}
	a.BaseElement = core.NewBaseElement(DataKey, keyFilter{a}, logger)
	first := func(cfg *Engine) error {
		cfg.defaultConcurrency = defaultConcurrency()
		cfg.afterLoad = a.dataLoaded
		return nil
	}
	engine, err := New(append([]EngineOptions{first}, opts...)...)
	if err != nil {
		return nil, err
	}
	a.engine = engine
	a.names = namesOf(engine.names.Load())
	a.positions = make(map[string]int, len(a.names))
	for i, name := range a.names {
		a.names[i] = strings.ToLower(name)
		a.positions[a.names[i]] = i
	}
	return a, nil
}

// defaultConcurrency is the number of processors, as far as the native code
// can be given it.
func defaultConcurrency() uint16 {
	processors := runtime.NumCPU()
	if processors > maxConcurrency {
		return maxConcurrency
	}
	return uint16(processors)
}

// acceptedKey is an Evidence key the data file lists, with its name as a
// Flow Data writes it, such as query.client-ip.
type acceptedKey struct {
	key  ipi_interop.EvidenceKey
	name string
}

// dataLoaded works out again what depends on the data file, from the one the
// Engine has just loaded, being the accepted Evidence keys and the Property
// metadata.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L73-L94
func (a *AspectEngine) dataLoaded(engine *Engine) {
	keys := engine.manager.HeaderKeys()
	accepted := make([]acceptedKey, 0, len(keys))
	for _, key := range keys {
		if name := evidenceKeyName(key); name != "" {
			accepted = append(accepted, acceptedKey{key, name})
		}
	}
	a.accepted.Store(&accepted)

	loaded, err := engine.manager.Properties()
	if err != nil {
		a.Logger().Error("the Property metadata could not be read from "+
			"the data file", "engine", DataKey, "error", err)
		return
	}
	metadata := make([]core.PropertyMetadata, 0, len(loaded))
	for _, p := range loaded {
		metadata = append(metadata, core.PropertyMetadata{
			Name:                  p.Name,
			Element:               a,
			Category:              p.Category,
			Type:                  valueType,
			Available:             p.Available,
			DataTiersWherePresent: p.DataTiers,
			Description:           p.Description,
		})
	}
	a.metadata.Store(&metadata)
}

// Engine returns the Engine that loads the data file, for its metadata and
// for RefreshData. Close stops it, so do not call its Stop.
func (a *AspectEngine) Engine() *Engine {
	return a.engine
}

// Properties returns the metadata of every Property the data file in use
// defines. Available says whether the Engine was created with the Property,
// so that its AspectData has a value for it.
func (a *AspectEngine) Properties() []core.PropertyMetadata {
	if metadata := a.metadata.Load(); metadata != nil {
		return *metadata
	}
	return nil
}

// DataSourceTier returns an empty string, because the data file does not
// name its tier.
func (a *AspectEngine) DataSourceTier() string {
	return ""
}

// Process chooses the IP address from the Evidence, looks it up, and adds an
// AspectData under DataKey. It reads no Property value. Evidence that holds
// no address the Engine can read is not an error, and gives an AspectData
// whose Properties have no value. Once the Engine has stopped it gives an
// error, and Stop waits for a Process already running.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L108-L136
func (a *AspectEngine) Process(fd core.FlowData) error {
	if !a.engine.acquire() {
		return errEngineStopped
	}
	defer a.engine.release()
	data, err := a.processInto(
		ipi_interop.NewResultsIpi(a.engine.manager), fd.Evidence())
	if err != nil {
		return err
	}
	aspect := newAspectData(a, data)
	held := fd.GetOrAdd(DataKey, func(core.FlowData) core.ElementData {
		return aspect
	})
	if held != core.ElementData(aspect) {
		// Another element has added Element Data under the key, so this one
		// is not held and is closed here.
		aspect.Close()
	}
	return nil
}

// Close stops the Engine and frees the data file. It is safe to call more
// than once.
func (a *AspectEngine) Close() error {
	a.closeOnce.Do(a.engine.Stop)
	return nil
}

// keyFilter accepts the Evidence keys the data file in use lists, as they
// were when a data file was last loaded, so that it follows a data refresh.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L73-L88
type keyFilter struct {
	engine *AspectEngine
}

// Include reports whether the Engine accepts the Evidence key, ignoring
// case.
func (f keyFilter) Include(key string) bool {
	accepted := f.engine.accepted.Load()
	if accepted == nil {
		return false
	}
	for _, k := range *accepted {
		if strings.EqualFold(key, k.name) {
			return true
		}
	}
	return false
}

// evidenceKeyName gives an accepted Evidence key as a Flow Data writes it,
// such as query.client-ip, or an empty string for a prefix the Engine does
// not read.
func evidenceKeyName(key ipi_interop.EvidenceKey) string {
	switch key.Prefix {
	case ipi_interop.HttpEvidenceQuery:
		return queryPrefix + "." + key.Key
	case ipi_interop.HttpEvidenceServer:
		return serverPrefix + "." + key.Key
	default:
		return ""
	}
}

// evidenceResults is the part of ipi_interop.ResultsIpi that processing
// Evidence uses.
type evidenceResults interface {
	nativeResults
	ResultsIpiFromEvidence(evidence *ipi_interop.Evidence) error
}

// processInto passes the values of the accepted Evidence keys to the native
// code, which chooses the IP address from them, and wraps the results in a
// Data, reading no value. It frees results when processing fails.
func (a *AspectEngine) processInto(results evidenceResults, evidence core.Evidence) (*Data, error) {
	var accepted []acceptedKey
	if loaded := a.accepted.Load(); loaded != nil {
		accepted = *loaded
	}
	native := ipi_interop.NewEvidenceWithCapacity(len(accepted))
	defer native.Free()
	for _, k := range accepted {
		value, ok := evidence.Get(k.name)
		if !ok || value == nil {
			continue
		}
		text, isText := value.(string)
		if !isText {
			text = fmt.Sprint(value)
		}
		if err := native.Add(k.key.Prefix, k.key.Key, text); err != nil {
			results.Free()
			return nil, err
		}
	}
	if err := results.ResultsIpiFromEvidence(native); err != nil {
		results.Free()
		return nil, err
	}
	return a.engine.newData(results), nil
}
