# Tests for the build selection of a pull request, with Pester 5: the modules
# ci/products.ps1 builds and the ci/options.json entries ci/select-options.ps1
# chooses. The last tests run common-ci's steps/select-options.ps1, so they need
# a checkout of common-ci, named by COMMON_CI or beside this repository.
#
#   $env:COMMON_CI = '../common-ci'; Invoke-Pester ci/select-options.Tests.ps1

BeforeAll {
    . (Join-Path $PSScriptRoot 'products.ps1')

    $script:Select = Join-Path $PSScriptRoot 'select-options.ps1'
    $script:OptionsJson = Get-Content -Raw (Join-Path $PSScriptRoot 'options.json')
    $script:NotMacOS = @((ConvertFrom-Json $script:OptionsJson) |
        Where-Object { $_.Image -notlike 'macos*' } | ForEach-Object Name)
    $script:CommonCi = @(
        $env:COMMON_CI,
        (Join-Path $PSScriptRoot '..' '..' 'common-ci')) |
        Where-Object { $_ -and (Test-Path (Join-Path $_ 'steps' 'select-options.ps1')) } |
        Select-Object -First 1

    function Save-File([string]$Repo, [string]$File, [string]$Text) {
        $full = Join-Path $Repo $File
        $null = New-Item -ItemType Directory -Force (Split-Path $full)
        Set-Content -Path $full -Value $Text -NoNewline
        git -C $Repo add -A
        git -C $Repo commit -q -m $File
    }

    # A repository whose rc holds a change to common that main does not, as rc
    # does between releases, and a pull request into rc that changes the given
    # files. It carries this folder's select-options.ps1 and options.json, as
    # common-ci's selection step reads them from the checkout.
    function New-PullRequestRepo([string[]]$Files) {
        $repo = Join-Path ([IO.Path]::GetTempPath()) "select-$([guid]::NewGuid())"
        git init -q -b main $repo
        git -C $repo config user.email 'tests@example.com'
        git -C $repo config user.name 'Tests'
        git -C $repo config commit.gpgsign false
        Save-File $repo 'ci/options.json' $script:OptionsJson
        if (Test-Path $script:Select) {
            Save-File $repo 'ci/select-options.ps1' (Get-Content -Raw $script:Select)
        }
        git -C $repo update-ref refs/remotes/origin/main HEAD
        git -C $repo checkout -q -b rc
        Save-File $repo 'common/common.go' 'package common'
        git -C $repo update-ref refs/remotes/origin/rc HEAD
        git -C $repo checkout -q -b pull-request
        foreach ($file in $Files) { Save-File $repo $file "// $file" }
        return $repo
    }
}

Describe 'Resolve-AffectedProducts' {
    It 'builds <Product> and what requires it for a change to <File>' -TestCases @(
        @{ File = 'pipeline/core/flowdata.go'; Product = 'pipeline'
            Expected = @('ip-intelligence', 'pipeline') }
        @{ File = 'examples/pipeline/1_simple_element/main.go'; Product = 'pipeline'
            Expected = @('ip-intelligence', 'pipeline') }
        @{ File = 'device-detection/dd/results_hash.go'; Product = 'device-detection'
            Expected = @('device-detection') }
        @{ File = 'ip-intelligence/ipi_cloud/engine.go'; Product = 'ip-intelligence'
            Expected = @('ip-intelligence') }
    ) {
        (Resolve-AffectedProducts @($File) | Sort-Object) -join ',' |
            Should -Be ($Expected -join ',')
    }

    It 'builds every product for a change to <File>' -TestCases @(
        @{ File = 'common/file_pulling.go' }
        @{ File = 'ci/build-project.ps1' }
        @{ File = 'go.work' }
    ) {
        (Resolve-AffectedProducts @($File) | Sort-Object) -join ',' |
            Should -Be ($AllProducts -join ',')
    }
}

Describe 'Get-AffectedProducts' {
    AfterEach { if ($repo) { Remove-Item -Recurse -Force $repo } }

    It 'measures a pull request into rc from rc' {
        $repo = New-PullRequestRepo @('device-detection/dd/results_hash.go')
        (Get-AffectedProducts $repo 'rc') -join ',' | Should -Be 'device-detection'
    }

    It 'measured from main, takes in the change rc holds' {
        $repo = New-PullRequestRepo @('device-detection/dd/results_hash.go')
        (Get-AffectedProducts $repo 'main' | Sort-Object) -join ',' |
            Should -Be ($AllProducts -join ',')
    }

    It 'builds every product for a build of rc itself' {
        $repo = New-PullRequestRepo @('device-detection/dd/results_hash.go')
        git -C $repo checkout -q rc
        (Get-AffectedProducts $repo 'rc' | Sort-Object) -join ',' |
            Should -Be ($AllProducts -join ',')
    }
}

Describe 'select-options.ps1' {
    It 'options.json has a macOS entry to leave out' {
        $script:NotMacOS.Count | Should -BeLessThan @(ConvertFrom-Json $script:OptionsJson).Count
    }

    It 'keeps every entry but macOS, with performance off' {
        $entries = @(ConvertFrom-Json $script:OptionsJson -AsHashtable)
        $chosen = @(& $script:Select -Options $entries `
            -ChangedFiles @('pipeline/core/flowdata.go'))
        $chosen | ForEach-Object { $_ | Should -BeOfType [System.Collections.IDictionary] }
        $chosen.Name -join ',' | Should -Be ($script:NotMacOS -join ',')
        @($chosen | Where-Object { $_.RunPerformance }).Count | Should -Be 0
    }
}

Describe "common-ci's selection step" {
    BeforeAll {
        if (!$script:CommonCi) {
            throw 'Name a checkout of common-ci with COMMON_CI'
        }
        $script:Step = Join-Path $script:CommonCi 'steps' 'select-options.ps1'
    }
    AfterEach { if ($repo) { Remove-Item -Recurse -Force $repo } }

    It 'builds a pull request into rc on every entry but macOS' {
        $repo = New-PullRequestRepo @('pipeline/core/flowdata.go')
        & $script:Step -RepoName $repo -Branch rc -Options $script:OptionsJson `
            -SetVariable Chosen
        $names = @(ConvertFrom-Json $Chosen | ForEach-Object Name)
        $names -join ',' | Should -Be ($script:NotMacOS -join ',')
        @(ConvertFrom-Json $Chosen | Where-Object RunPerformance).Count | Should -Be 0
    }

    It 'builds nothing for a pull request that changes only documentation' {
        $repo = New-PullRequestRepo @('pipeline/README.md')
        & $script:Step -RepoName $repo -Branch rc -Options $script:OptionsJson `
            -SetVariable Chosen
        $Chosen | Should -Be '[]'
    }
}
