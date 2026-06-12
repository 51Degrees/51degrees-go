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

package core

import (
	"encoding/json"
	"log/slog"
	"os"
	"strings"
)

// This file implements building a Pipeline from a configuration object, which
// can in turn be loaded from a human-readable file. The structure intentionally
// mirrors PipelineOptions / ElementOptions from the reference implementations so
// configuration files stay consistent across languages.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/pipeline-configuration.md

// ElementOptions describes how to build a single Flow Element. If SubElements is
// populated, the element is a ParallelElements group and BuilderName /
// BuildParameters are ignored.
type ElementOptions struct {
	// BuilderName is the registered name of the element builder.
	BuilderName string `json:"builderName"`
	// BuildParameters supplies named configuration values for the builder. Only
	// the simple types described in the pipeline-configuration specification are
	// supported (string, boolean, numeric, comma-separated list, enumeration).
	BuildParameters map[string]any `json:"buildParameters,omitempty"`
	// SubElements, if populated, defines the children of a ParallelElements group.
	SubElements []ElementOptions `json:"subElements,omitempty"`
}

// PipelineOptions describes how to build a Pipeline.
type PipelineOptions struct {
	// Elements lists the elements to add, in flow order.
	Elements []ElementOptions `json:"elements"`
	// BuildParameters supplies Pipeline-level configuration. Recognized keys
	// (case-insensitive) include "suppressProcessExceptions" and "concurrent".
	BuildParameters map[string]any `json:"buildParameters,omitempty"`
}

// ElementFactory creates a Flow Element from named build parameters.
type ElementFactory func(params map[string]any, logger *slog.Logger) (FlowElement, error)

// BuilderRegistry maps builder names to factories so that Pipelines can be
// constructed from configuration.
type BuilderRegistry struct {
	factories map[string]ElementFactory
}

// NewBuilderRegistry creates an empty registry.
func NewBuilderRegistry() *BuilderRegistry {
	return &BuilderRegistry{factories: make(map[string]ElementFactory)}
}

// Register adds a factory under the given builder name. Names are matched
// case-insensitively. A configuration BuilderName also matches if appending
// "Builder" to it yields the registered name (convention-based matching).
func (r *BuilderRegistry) Register(name string, factory ElementFactory) {
	r.factories[lower(name)] = factory
}

func (r *BuilderRegistry) lookup(builderName string) (ElementFactory, bool) {
	if f, ok := r.factories[lower(builderName)]; ok {
		return f, true
	}
	if f, ok := r.factories[lower(builderName)+"builder"]; ok {
		return f, true
	}
	return nil, false
}

// BuildFromConfiguration constructs a Pipeline from the given options using the
// registered factories.
func (r *BuilderRegistry) BuildFromConfiguration(opts PipelineOptions, logger *slog.Logger) (Pipeline, error) {
	if logger == nil {
		logger = slog.Default()
	}
	builder := NewPipelineBuilder().SetLogger(logger)

	if v, ok := boolParam(opts.BuildParameters, "suppressProcessExceptions"); ok {
		builder.SetSuppressProcessExceptions(v)
	}
	if v, ok := boolParam(opts.BuildParameters, "concurrent"); ok {
		builder.SetConcurrent(v)
	}

	for _, eo := range opts.Elements {
		if len(eo.SubElements) > 0 {
			children := make([]FlowElement, 0, len(eo.SubElements))
			for _, se := range eo.SubElements {
				child, err := r.buildElement(se, logger)
				if err != nil {
					return nil, err
				}
				children = append(children, child)
			}
			builder.AddParallel(children...)
			continue
		}
		el, err := r.buildElement(eo, logger)
		if err != nil {
			return nil, err
		}
		builder.AddFlowElement(el)
	}
	return builder.Build()
}

func (r *BuilderRegistry) buildElement(eo ElementOptions, logger *slog.Logger) (FlowElement, error) {
	factory, ok := r.lookup(eo.BuilderName)
	if !ok {
		return nil, NewConfigurationError("no builder registered for '"+eo.BuilderName+"'", nil)
	}
	el, err := factory(eo.BuildParameters, logger)
	if err != nil {
		return nil, NewConfigurationError("failed to build element '"+eo.BuilderName+"'", err)
	}
	return el, nil
}

// LoadPipelineOptions reads PipelineOptions from a JSON file.
func LoadPipelineOptions(path string) (PipelineOptions, error) {
	var opts PipelineOptions
	data, err := os.ReadFile(path)
	if err != nil {
		return opts, NewConfigurationError("could not read configuration file '"+path+"'", err)
	}
	if err := json.Unmarshal(data, &opts); err != nil {
		return opts, NewConfigurationError("could not parse configuration file '"+path+"'", err)
	}
	return opts, nil
}

// boolParam looks up a boolean build parameter by name (case-insensitive). It
// accepts native booleans and the strings "true"/"false".
func boolParam(params map[string]any, name string) (bool, bool) {
	for k, v := range params {
		if !strings.EqualFold(k, name) {
			continue
		}
		switch t := v.(type) {
		case bool:
			return t, true
		case string:
			return strings.EqualFold(t, "true"), true
		}
	}
	return false, false
}
