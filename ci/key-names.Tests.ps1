# Tests that the CI scripts read and set only _51DEGREES_ names, and fetch the
# evidence as YAML, with Pester 5.
#
#   Invoke-Pester ci/key-names.Tests.ps1
#
# The workflows pass their secrets to common-ci with 'secrets: inherit', and
# common-ci puts the keys into the environment under those names before any
# script of this repository runs. Its fetch-assets step downloads every data
# file from the Distributor with _51DEGREES_LICENSE_KEY. A script here that
# sets _51DEGREES_LICENSE_KEY clears the key common-ci set.

BeforeDiscovery {
    $Scripts = @('fetch-assets.ps1', 'run-integration-tests.ps1')
    $All = @(Get-ChildItem $PSScriptRoot -Filter '*.ps1' | ForEach-Object Name)
}

BeforeAll {
    # The names that Go, Git, GitHub, PowerShell and the operating system
    # define, which are read and set as each requires.
    $script:Platform = '^(CI|PATH|GOWORK|GOPRIVATE|CGO_ENABLED|CGO_LDFLAGS|' +
        'PSModulePath|ProgramFiles|GITHUB_\w+|GIT_CONFIG_\w+)$'

    function Get-ScriptAst([string]$Name) {
        $path = Join-Path $PSScriptRoot $Name
        $tokens = $errors = $null
        $ast = [Management.Automation.Language.Parser]::ParseFile(
            $path, [ref]$tokens, [ref]$errors)
        $errors | Should -BeNullOrEmpty
        return $ast
    }

    # The environment variables the script reads or sets, by name.
    function Get-NamedVariables($Ast) {
        $Ast.FindAll({
                param($node)
                $node -is [Management.Automation.Language.VariableExpressionAst] -and
                $node.VariablePath.DriveName -eq 'env'
            }, $true) | ForEach-Object {
            $_.VariablePath.UserPath -replace '^env:', ''
        } | Select-Object -Unique
    }

    # The environment variables the script sets, by name.
    function Get-SetVariables($Ast) {
        $Ast.FindAll({
                param($node)
                $node -is [Management.Automation.Language.AssignmentStatementAst] -and
                $node.Left -is [Management.Automation.Language.VariableExpressionAst] -and
                $node.Left.VariablePath.DriveName -eq 'env'
            }, $true) | ForEach-Object {
            $_.Left.VariablePath.UserPath -replace '^env:', ''
        }
    }

    # The files the script names, which are the strings that end with a file
    # extension and hold no folder.
    function Get-NamedFiles($Ast) {
        $Ast.FindAll({
                param($node)
                $node -is [Management.Automation.Language.StringConstantExpressionAst]
            }, $true) | ForEach-Object Value |
            Where-Object { $_ -match '^[^/\\$]+\.[A-Za-z0-9]+$' }
    }
}

Describe 'The CI scripts' {
    It '<_> reads and sets only _51DEGREES_ and platform names' -ForEach $All {
        $named = @(Get-NamedVariables (Get-ScriptAst $_))
        $others = @($named | Where-Object {
                $_ -notlike '_51DEGREES_*' -and $_ -notmatch $script:Platform
            })
        $others | Should -BeNullOrEmpty
    }

    It '<_> leaves _51DEGREES_LICENSE_KEY as common-ci set it' -ForEach $Scripts {
        $set = @(Get-SetVariables (Get-ScriptAst $_))
        $set | Should -Not -Contain '_51DEGREES_LICENSE_KEY'
    }
}

Describe 'fetch-assets.ps1' {
    It 'fetches only data files and YAML evidence' {
        $files = @(Get-NamedFiles (Get-ScriptAst 'fetch-assets.ps1') |
                Where-Object { $_ -notlike '*.ps1' } | Select-Object -Unique)
        $files.Count | Should -BeGreaterThan 0
        $others = @($files | Where-Object { $_ -notmatch '\.(hash|ipi|yml)$' })
        $others | Should -BeNullOrEmpty
    }
}
