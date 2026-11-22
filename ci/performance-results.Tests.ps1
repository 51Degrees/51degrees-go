# Tests, with Pester 5, that ci/run-performance-tests.ps1 publishes its figures
# under the name of the ci/options.json entry it ran for. common-ci's
# performance comparison reads results_<Name>.json for each entry, so figures
# under any other name are not compared and the entry's graph stops. The first
# tests run common-ci's steps/publish-performance-results.ps1, so they need a
# checkout of common-ci, named by COMMON_CI or beside this repository.
#
#   $env:COMMON_CI = '../common-ci'; Invoke-Pester ci/performance-results.Tests.ps1

BeforeAll {
    $script:Run = Join-Path $PSScriptRoot 'run-performance-tests.ps1'
    $script:CommonCi = @(
        $env:COMMON_CI,
        (Join-Path $PSScriptRoot '..' '..' 'common-ci')) |
        Where-Object { $_ -and (Test-Path (Join-Path $_ 'steps' 'publish-performance-results.ps1')) } |
        Select-Object -First 1

    # Stands in for the Go command, so the test needs neither the data files
    # nor the time the examples take. A performance example writes its figures
    # to the file named after -json-output, and the JavaScript Builder's
    # benchmarks print theirs.
    function go {
        $output = [array]::IndexOf($args, '-json-output')
        if ($args[0] -eq 'run' -and $output -ge 0) {
            Set-Content -Path $args[$output + 1] -Value (
                '{"HigherIsBetter":{"DetectionsPerSecond":2000},' +
                '"LowerIsBetter":{"AvgMillisecsPerDetection":0.5}}')
        } elseif ($args[0] -eq 'test') {
            'BenchmarkJavaScriptBuilder/Minified-4         	    2416	    500000 ns/op'
            'BenchmarkJavaScriptBuilder/NotMinified-4      	    3891	    250000 ns/op'
            'BenchmarkJavaScriptBuilderMinification-4      	    7059	    125000 ns/op'
        }
        $global:LASTEXITCODE = 0
    }

    # A folder laid out as a CI job's is: common-ci's steps, beside a checkout
    # of this repository that is not a pull request, so every product runs.
    function New-Job {
        $job = Join-Path ([IO.Path]::GetTempPath()) "performance-$([guid]::NewGuid())"
        $steps = New-Item -ItemType Directory -Force (Join-Path $job 'steps')
        Copy-Item (Join-Path $script:CommonCi 'steps' 'publish-performance-results.ps1') $steps
        $repo = Join-Path $job '51degrees-go'
        git init -q -b main $repo
        git -C $repo config user.email 'tests@example.com'
        git -C $repo config user.name 'Tests'
        git -C $repo config commit.gpgsign false
        foreach ($folder in 'examples/device-detection', 'examples/ip-intelligence', 'pipeline') {
            $null = New-Item -ItemType Directory -Force (Join-Path $repo $folder)
            Set-Content -Path (Join-Path $repo $folder 'go.mod') -Value 'module example'
        }
        git -C $repo add -A
        git -C $repo commit -q -m 'Modules'
        git -C $repo update-ref refs/remotes/origin/main HEAD
        return $job
    }

    # Runs the script in the job folder for the entry, and gives the files it
    # published.
    function Publish-Figures([string]$Job, [string]$Name) {
        $path = $env:_51DEGREES_IPI_PATH
        Push-Location $Job
        try {
            $null = & $script:Run -RepoName '51degrees-go' -Name $Name 6>&1
        } finally {
            Pop-Location
            $env:_51DEGREES_IPI_PATH = $path
        }
        Get-ChildItem (Join-Path $Job '51degrees-go' 'test-results' 'performance-summary')
    }
}

Describe 'run-performance-tests.ps1' {
    BeforeAll {
        if (!$script:CommonCi) {
            throw 'Name a checkout of common-ci with COMMON_CI'
        }
    }
    AfterEach { if ($job) { Remove-Item -Recurse -Force $job } }

    It 'publishes one results file, named for the options entry <Name>' -TestCases @(
        @{ Name = 'Ubuntu_Go_1.24' }
        @{ Name = 'Windows_Go_1.24' }
    ) {
        $job = New-Job
        $published = @(Publish-Figures $job $Name)
        $published.Name -join ',' | Should -Be "results_$Name.json"
    }

    It 'publishes the figures of every engine and of the JavaScript Builder' {
        $job = New-Job
        $published = @(Publish-Figures $job 'Ubuntu_Go_1.24')
        $figures = Get-Content -Raw $published[0].FullName | ConvertFrom-Json -AsHashtable
        @($figures.HigherIsBetter.Keys | Sort-Object) -join ',' | Should -Be (
            'DetectionsPerSecond,LookupsPerSecond,MinificationsPerSecond,' +
            'MinifiedScriptsPerSecond,ScriptsNotMinifiedPerSecond')
        @($figures.LowerIsBetter.Keys | Sort-Object) -join ',' | Should -Be (
            'AvgMillisecsPerDetection,AvgMillisecsPerLookup,AvgMillisecsPerMinification,' +
            'AvgMillisecsPerMinifiedScript,AvgMillisecsPerScriptNotMinified')
        $figures.HigherIsBetter.MinificationsPerSecond | Should -Be 8000
    }
}

# PowerShell names take no account of case, so a loop over $name writes to a
# parameter called $Name, and everything after the loop reads the last item in
# its place.
Describe 'The CI scripts' {
    It 'never use one of their own parameters as a loop variable' {
        $reused = foreach ($file in Get-ChildItem $PSScriptRoot -Filter *.ps1) {
            $script = [System.Management.Automation.Language.Parser]::ParseFile(
                $file.FullName, [ref]$null, [ref]$null)
            if (!$script.ParamBlock) { continue }
            $parameters = @($script.ParamBlock.Parameters |
                ForEach-Object { $_.Name.VariablePath.UserPath })
            # Only the script's own loops, as a function has its own variables.
            $loops = $script.FindAll({
                param($node)
                $node -is [System.Management.Automation.Language.ForEachStatementAst]
            }, $false)
            foreach ($loop in $loops) {
                $variable = $loop.Variable.VariablePath.UserPath
                if ($parameters -contains $variable) {
                    "$($file.Name) line $($loop.Extent.StartLineNumber) loops over `$$variable"
                }
            }
        }
        $reused -join '; ' | Should -BeNullOrEmpty
    }
}
