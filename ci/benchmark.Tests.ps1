# Tests, with Pester 5, for reading the figures out of go test -bench output,
# which ci/run-performance-tests.ps1 publishes for the JavaScript Builder.
#
#   Invoke-Pester ci/benchmark.Tests.ps1

BeforeAll {
    . (Join-Path $PSScriptRoot 'products.ps1')
}

Describe 'ConvertFrom-GoBenchmark' {
    It 'gives each benchmark in milliseconds, without the processor count' {
        $figures = ConvertFrom-GoBenchmark @(
            'goos: linux',
            'goarch: amd64',
            'BenchmarkJavaScriptBuilder/Minified-4         	    2416	    461250 ns/op',
            'BenchmarkJavaScriptBuilder/NotMinified-4      	    3891	    304000 ns/op',
            'BenchmarkJavaScriptBuilderMinification-4      	    7059	    165500.5 ns/op',
            'PASS',
            'ok  	github.com/51Degrees/51degrees-go/pipeline/v4/elements/javascriptbuilder	6.012s')
        $figures.Count | Should -Be 3
        $figures['JavaScriptBuilder/Minified'] | Should -Be 0.46125
        $figures['JavaScriptBuilder/NotMinified'] | Should -Be 0.304
        $figures['JavaScriptBuilderMinification'] | Should -Be 0.1655005
    }

    It 'reads a benchmark run on one processor, which has no count' {
        $figures = ConvertFrom-GoBenchmark @(
            'BenchmarkJavaScriptBuilderMinification 	    7059	    200000 ns/op')
        $figures['JavaScriptBuilderMinification'] | Should -Be 0.2
    }

    It 'gives nothing when no benchmark ran' {
        $figures = ConvertFrom-GoBenchmark @(
            'PASS',
            'ok  	github.com/51Degrees/51degrees-go/pipeline/v4/elements/javascriptbuilder	0.004s')
        $figures.Count | Should -Be 0
    }
}
