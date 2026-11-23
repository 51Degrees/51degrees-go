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

package javascriptbuilder

import (
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/jsonbuilder"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/sequence"
)

// The performance tests of the JavaScript Builder. They measure the time it
// takes to make the script with minification and without, and the time the
// minification alone takes, so the figures show the impact minification has.
// CI's performance run (ci/run-performance-tests.ps1) records each figure as a
// time and as a rate a second, and the Compare Performance job checks each
// rate against earlier runs, so a later change that makes one worse is seen.
// They set no time limit of their own, as a limit would depend on the runner.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/javascript-builder.md?plain=1#L109-L114

// benchmarkPipeline gives a Pipeline that makes the script, minified or not.
func benchmarkPipeline(b *testing.B, minify bool) core.Pipeline {
	b.Helper()
	p, err := core.NewPipelineBuilder().
		AddFlowElement(sequence.New(nil)).
		AddFlowElement(jsonbuilder.New(nil, nil)).
		AddFlowElement(NewBuilder().
			SetEndpoint("/51dpipeline/json").
			SetMinify(minify).
			Build()).
		Build()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { p.Close() })
	return p
}

// processOnce makes the script for one request, as a web server would.
func processOnce(b *testing.B, p core.Pipeline) string {
	fd := p.CreateFlowData()
	fd.AddEvidence("header.host", "example.com")
	fd.AddEvidence("header.protocol", "https")
	if err := fd.Process(); err != nil {
		b.Fatal(err)
	}
	data, err := core.Get[*Data](fd)
	if err != nil {
		b.Fatal(err)
	}
	return data.JavaScript()
}

// BenchmarkJavaScriptBuilder measures making the script for one request,
// with minification and without.
func BenchmarkJavaScriptBuilder(b *testing.B) {
	for _, c := range []struct {
		name   string
		minify bool
	}{{"Minified", true}, {"NotMinified", false}} {
		b.Run(c.name, func(b *testing.B) {
			p := benchmarkPipeline(b, c.minify)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				processOnce(b, p)
			}
		})
	}
}

// BenchmarkJavaScriptBuilderMinification measures the minification alone,
// of the script the builder makes before minifying it.
func BenchmarkJavaScriptBuilderMinification(b *testing.B) {
	script := processOnce(b, benchmarkPipeline(b, false))
	b.SetBytes(int64(len(script)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lightMinify(script)
	}
}
