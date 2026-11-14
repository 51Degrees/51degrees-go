# Tests for the release plan of ci/packages.ps1, with Pester 5. They need a
# checkout of common-ci, named by COMMON_CI or beside this repository.
#
#   $env:COMMON_CI = '../common-ci'; Invoke-Pester ci/packages.Tests.ps1
#
# Each test builds a throwaway repository holding the four modules, this
# repository's ci/packages.json and Go tags at v4.5.180, commits a change and
# reads the plan.

BeforeAll {
    . (Join-Path $PSScriptRoot 'packages.ps1')

    $script:PackagesJson = Join-Path $PSScriptRoot 'packages.json'
    $script:ModulePrefix = 'github.com/51Degrees/51degrees-go'

    # The go.mod of a module requiring the given modules, each at the version
    # given for it.
    function Get-GoModText([string]$Module, [hashtable]$Requires = @{}) {
        $lines = @("module $script:ModulePrefix/$Module/v4", '', 'go 1.22', '')
        foreach ($required in ($Requires.Keys | Sort-Object)) {
            $lines += "require $script:ModulePrefix/$required/v4 " +
                "v$($Requires[$required])"
        }
        return ($lines -join "`n") + "`n"
    }

    function Write-File([string]$Repo, [string]$File, [string]$Text) {
        $full = Join-Path $Repo $File
        $null = New-Item -ItemType Directory -Force (Split-Path $full)
        Set-Content -Path $full -Value $Text -NoNewline
    }

    # Writes every go.mod, with the dependents requiring common and pipeline
    # at the given versions.
    function Set-GoMods([string]$Repo, [string]$Common, [string]$Pipeline) {
        Write-File $Repo 'common/go.mod' (Get-GoModText 'common')
        Write-File $Repo 'pipeline/go.mod' (Get-GoModText 'pipeline')
        Write-File $Repo 'device-detection/go.mod' `
            (Get-GoModText 'device-detection' @{ common = $Common })
        Write-File $Repo 'ip-intelligence/go.mod' (Get-GoModText `
            'ip-intelligence' @{ common = $Common; pipeline = $Pipeline })
    }

    function Save-Commit([string]$Repo) {
        git -C $Repo add -A
        git -C $Repo -c user.name=test -c user.email=test@example.com `
            commit --quiet --allow-empty -m 'Change'
        return git -C $Repo rev-parse HEAD
    }

    # Changes each file and commits, returning the commit.
    function Add-Commit([string]$Repo, [string[]]$Files) {
        foreach ($file in $Files) {
            Write-File $Repo $file ([Guid]::NewGuid().ToString())
        }
        return Save-Commit $Repo
    }

    # A repository with every module's source, test, test data and docs, and
    # each module's Go tag v4.5.180 on that commit.
    function New-Repo {
        $repo = Join-Path ([IO.Path]::GetTempPath()) `
            ([Guid]::NewGuid().ToString())
        git init --quiet $repo
        Write-File $repo 'ci/packages.json' (Get-Content -Raw $script:PackagesJson)
        Set-GoMods $repo '4.5.180' '4.5.180'
        $null = Add-Commit $repo @(
            'common/errors.go', 'common/errors_test.go', 'common/mock_hash.gz',
            'common/README.MD', 'common/LICENSE',
            'pipeline/core/util.go', 'pipeline/core/core_test.go',
            'pipeline/README.md',
            'device-detection/dd/constants.go',
            'device-detection/dd/helpers_test.go',
            'device-detection/onpremise/mock_hash.gz',
            'ip-intelligence/ipi_onpremise/ipi.go',
            'ip-intelligence/docs/Doxyfile',
            'examples/device-detection/getting_started/main.go')
        foreach ($module in 'common', 'pipeline', 'device-detection',
                'ip-intelligence') {
            git -C $repo tag "$module/v4.5.180"
        }
        return $repo
    }

    # The names and versions the plan releases, such as 'common 4.5.181', in
    # the plan's order.
    function Get-Planned([string]$Repo) {
        return @(Get-ModuleReleasePlan $Repo | ForEach-Object {
            "$($_.Name) $($_.Version)"
        })
    }
}

Describe 'Get-ModuleReleasePlan' {

    AfterEach {
        if ($script:Repo -and (Test-Path $script:Repo)) {
            Remove-Item -Recurse -Force $script:Repo
        }
        $script:Repo = $null
    }

    It 'releases a module whose source changed, with its dependents' {
        $script:Repo = New-Repo
        $null = Add-Commit $script:Repo @('common/errors.go')
        Get-Planned $script:Repo | Should -Be @(
            'common 4.5.181', 'device-detection 4.5.181',
            'ip-intelligence 4.5.181')
    }

    It 'releases nothing for a change to <What> alone' -ForEach @(
        @{ What = 'a test'; Files = @('device-detection/dd/helpers_test.go') }
        @{ What = 'test data'; Files = @('device-detection/onpremise/mock_hash.gz',
            'common/mock_hash.gz') }
        @{ What = 'a readme'; Files = @('pipeline/README.md') }
        @{ What = 'a readme named in capitals'; Files = @('common/README.MD') }
        @{ What = 'a licence'; Files = @('common/LICENSE') }
        @{ What = 'the documentation'; Files = @('ip-intelligence/docs/Doxyfile') }
        @{ What = 'an example'; Files = @(
            'examples/device-detection/getting_started/main.go') }
    ) {
        $script:Repo = New-Repo
        $null = Add-Commit $script:Repo $Files
        Get-Planned $script:Repo | Should -BeNullOrEmpty
    }

    It 'releases a module whose test and source both changed' {
        $script:Repo = New-Repo
        $null = Add-Commit $script:Repo @('pipeline/core/core_test.go',
            'pipeline/core/util.go')
        Get-Planned $script:Repo | Should -Be @(
            'pipeline 4.5.181', 'ip-intelligence 4.5.181')
    }

    # After a release of common, its dependents are tagged on commits that
    # raise their go.mod, which the branch only gets from the nightly package
    # update. A test changed after that is still not a change.
    It 'releases nothing for a test changed after the raise reached the branch' {
        $script:Repo = New-Repo
        $released = Add-Commit $script:Repo @('common/errors.go')
        git -C $script:Repo tag 'common/v4.5.181'
        Set-GoMods $script:Repo '4.5.181' '4.5.180'
        $null = Save-Commit $script:Repo
        git -C $script:Repo tag 'device-detection/v4.5.181'
        git -C $script:Repo tag 'ip-intelligence/v4.5.181'

        # The branch is still at the released commit, then the nightly
        # raises the requirements on it and a test changes.
        git -C $script:Repo reset --quiet --hard $released
        Set-GoMods $script:Repo '4.5.181' '4.5.180'
        $null = Save-Commit $script:Repo
        $null = Add-Commit $script:Repo @('device-detection/dd/helpers_test.go')

        Get-Planned $script:Repo | Should -BeNullOrEmpty
    }

    It 'releases nothing for a test changed before the raise reached the branch' {
        $script:Repo = New-Repo
        $released = Add-Commit $script:Repo @('common/errors.go')
        git -C $script:Repo tag 'common/v4.5.181'
        Set-GoMods $script:Repo '4.5.181' '4.5.180'
        $null = Save-Commit $script:Repo
        git -C $script:Repo tag 'device-detection/v4.5.181'
        git -C $script:Repo tag 'ip-intelligence/v4.5.181'

        git -C $script:Repo reset --quiet --hard $released
        $null = Add-Commit $script:Repo @('device-detection/dd/helpers_test.go')

        Get-Planned $script:Repo | Should -BeNullOrEmpty
    }

    # A module that needs a change to another in the same release requires
    # the version the plan releases of it, which has no tag yet.
    It 'releases a module that requires the version the plan releases of another' {
        $script:Repo = New-Repo
        Set-GoMods $script:Repo '4.5.180' '4.5.181'
        $null = Add-Commit $script:Repo @('pipeline/core/util.go')
        Get-Planned $script:Repo | Should -Be @(
            'pipeline 4.5.181', 'ip-intelligence 4.5.181')
    }

    It 'stops when a module requires a version with no tag that <What>' -ForEach @(
        @{ What = 'the plan does not release'; Pipeline = '4.5.181'
            Files = @(); Planned = 'does not release pipeline' }
        @{ What = 'is after the one planned'; Pipeline = '4.5.182'
            Files = @('pipeline/core/util.go'); Planned = 'releases pipeline 4.5.181' }
    ) {
        $script:Repo = New-Repo
        Set-GoMods $script:Repo '4.5.180' $Pipeline
        $null = Add-Commit $script:Repo $Files
        { Get-Planned $script:Repo } | Should -Throw ("ip-intelligence/go.mod " +
            "requires $script:ModulePrefix/pipeline/v4 v$Pipeline, which has no " +
            "tag pipeline/v$Pipeline, and this publish $Planned.")
    }
}

Describe 'The latest release of each module' {

    BeforeAll {
        . (Join-Path $PSScriptRoot 'products.ps1')
    }

    AfterEach {
        if ($script:Repo -and (Test-Path $script:Repo)) {
            Remove-Item -Recurse -Force $script:Repo
        }
        $script:Repo = $null
    }

    It 'is the highest version of the module''s own Go tags' {
        $script:Repo = New-Repo
        foreach ($tag in 'common/v4.5.99', 'common/v4.5.181',
                'common/4.5.200', 'common/v4.5.182-alpha.1',
                'common-extra/v4.5.190', 'pipeline/v4.5.183', '4.5.184') {
            git -C $script:Repo tag $tag
        }
        Get-ModuleRelease $script:Repo 'common' | Should -Be '4.5.181'
        Get-ModuleRelease $script:Repo 'pipeline' | Should -Be '4.5.183'
        Get-ModuleRelease $script:Repo 'device-detection' | Should -Be '4.5.180'
    }

    It 'is nothing for a module with no Go tag' {
        $script:Repo = New-Repo
        git -C $script:Repo tag -d 'common/v4.5.180' | Out-Null
        Get-ModuleRelease $script:Repo 'common' | Should -BeNullOrEmpty
    }

    # The go line an example needs is the highest of the modules it requires,
    # each read from its go.mod at its own latest tag.
    It 'gives the go line of each required module at its own latest tag' {
        $script:Repo = New-Repo
        Write-File $script:Repo 'pipeline/go.mod' `
            ((Get-GoModText 'pipeline') -replace 'go 1\.22', 'go 1.23')
        $null = Save-Commit $script:Repo
        git -C $script:Repo tag 'pipeline/v4.5.181'
        Write-File $script:Repo 'pipeline/go.mod' `
            ((Get-GoModText 'pipeline') -replace 'go 1\.22', 'go 1.25')
        Write-File $script:Repo 'examples/pipeline/go.mod' ((Get-GoModText `
            'pipeline' @{ common = '4.5.180'; pipeline = '4.5.180' }) -replace
            'module \S+', "module $script:ModulePrefix/examples/pipeline")
        $null = Save-Commit $script:Repo
        Get-ReleaseGoVersion $script:Repo 'examples/pipeline' | Should -Be '1.23'
    }
}
