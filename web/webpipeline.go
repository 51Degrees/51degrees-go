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

// Package web provides helpers for using the Pipeline API inside a net/http
// application: assembling a web-ready Pipeline, populating Evidence from a
// request, managing the Flow Data lifecycle, setting response headers, and
// serving the client-side JavaScript and JSON endpoints.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/web-integration.md
package web

import (
	"log/slog"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/elements/javascriptbuilder"
	"github.com/51Degrees/pipeline-go/elements/jsonbuilder"
	"github.com/51Degrees/pipeline-go/elements/sequence"
	"github.com/51Degrees/pipeline-go/elements/setheaders"
)

// PipelineConfig controls assembly of a web-ready Pipeline.
type PipelineConfig struct {
	// The Disable* options prevent the corresponding client-side element from
	// being added automatically, as required by the web-integration specification.

	// DisableSequence prevents the Sequence element being added automatically.
	DisableSequence bool
	// DisableSetHeaders prevents the Set Headers element being added automatically.
	DisableSetHeaders bool
	// DisableJSONBuilder prevents the JSON Builder element being added automatically.
	DisableJSONBuilder bool
	// DisableJavaScriptBuilder prevents the JavaScript Builder element being added automatically.
	DisableJavaScriptBuilder bool

	// Logger is used by any elements added automatically and by the Pipeline.
	Logger *slog.Logger

	// SuppressProcessExceptions configures the resulting Pipeline.
	SuppressProcessExceptions bool

	// JSONEndpoint is the client-side callback path baked into the generated
	// JavaScript (the JavaScript Builder's endpoint). It MUST match the path the
	// Middleware intercepts for JSON (see WithJSONURL). Defaults to
	// DefaultJSONURL. Without an endpoint the generated script cannot call back,
	// so the client-side evidence/refresh feature would be disabled.
	JSONEndpoint string
}

// AssemblePipeline builds a Pipeline containing the supplied elements plus the
// client-side elements required by the web integration, in the required order:
// the Sequence Element first, then the supplied elements, then the Set Headers
// Element, with the JSON Builder penultimate and the JavaScript Builder last.
// Any client-side element already present among the supplied elements (matched
// by data key) is not added again.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/web-integration.md#pipeline-configuration
func AssemblePipeline(cfg PipelineConfig, elements ...core.FlowElement) (core.Pipeline, error) {
	present := map[string]bool{}
	for _, e := range elements {
		present[e.DataKey()] = true
	}

	builder := core.NewPipelineBuilder().
		SetLogger(cfg.Logger).
		SetSuppressProcessExceptions(cfg.SuppressProcessExceptions)

	if !cfg.DisableSequence && !present[sequence.DataKey] {
		builder.AddFlowElement(sequence.New(cfg.Logger))
	}
	for _, e := range elements {
		builder.AddFlowElement(e)
	}
	if !cfg.DisableSetHeaders && !present[setheaders.DataKey] {
		builder.AddFlowElement(setheaders.New(cfg.Logger))
	}
	if !cfg.DisableJSONBuilder && !present[jsonbuilder.DataKey] {
		builder.AddFlowElement(jsonbuilder.New(cfg.Logger, nil))
	}
	if !cfg.DisableJavaScriptBuilder && !present[javascriptbuilder.DataKey] {
		jsonEndpoint := cfg.JSONEndpoint
		if jsonEndpoint == "" {
			jsonEndpoint = DefaultJSONURL
		}
		// Bake in the callback endpoint so the generated script can post results
		// back to the intercepted JSON URL. Without it, the client-side
		// evidence/refresh round-trip would be disabled.
		builder.AddFlowElement(javascriptbuilder.NewBuilder().
			SetLogger(cfg.Logger).
			SetEndpoint(jsonEndpoint).
			Build())
	}
	return builder.Build()
}
