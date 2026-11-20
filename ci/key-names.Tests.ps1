# Tests that the CI scripts take their keys only under the _51DEGREES_ names,
# with Pester 5.
#
#   Invoke-Pester ci/key-names.Tests.ps1
#
# The workflows pass their secrets to common-ci with 'secrets: inherit', and
# common-ci puts the keys into the environment under those names before any
# script of this repository runs. Its fetch-assets step downloads every data
# file from the Distributor with _51DEGREES_LICENSE_KEY, IP Intelligence as
# IPIV41 IPIV4Enterprise, so there is no address variable for a data file.
# A script here that takes a key under an old name, or sets
# _51DEGREES_LICENSE_KEY from one, gets nothing and clears the key common-ci
# set.

BeforeDiscovery {
    $Scripts = @('fetch-assets.ps1', 'run-integration-tests.ps1')
}

BeforeAll {
    $script:Root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path

    # The parameters the old secrets came in by.
    $script:OldParameters = @(
        'DeviceDetection', 'DeviceDetectionUrl', 'CsvUrl',
        'IpIntelligence', 'IpIntelligenceUrl'
    )

    # The dropped address variable, in two parts so that this file does not
    # name it.
    $script:DroppedUrl = '_51DEGREES_IPI_DATA' + '_FILE_URL'

    function Get-ScriptAst([string]$Name) {
        $path = Join-Path $PSScriptRoot $Name
        $tokens = $errors = $null
        $ast = [Management.Automation.Language.Parser]::ParseFile(
            $path, [ref]$tokens, [ref]$errors)
        $errors | Should -BeNullOrEmpty
        return $ast
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
}

Describe 'The CI scripts' {
    It '<_> takes no old key or address parameter' -ForEach $Scripts {
        $ast = Get-ScriptAst $_
        $parameters = @($ast.ParamBlock.Parameters |
                ForEach-Object { $_.Name.VariablePath.UserPath })
        foreach ($old in $script:OldParameters) {
            $parameters | Should -Not -Contain $old
        }
    }

    It '<_> leaves _51DEGREES_LICENSE_KEY as common-ci set it' -ForEach $Scripts {
        $set = @(Get-SetVariables (Get-ScriptAst $_))
        $set | Should -Not -Contain '_51DEGREES_LICENSE_KEY'
        $set | Should -Not -Contain $script:DroppedUrl
    }
}

Describe 'The dropped address variable' {
    It 'is not read, set or documented outside the C library' {
        $own = 'ci/key-names.Tests.ps1'
        $files = @(& git -C $script:Root -c core.quotePath=false ls-files |
                Where-Object { $_ -ne $own -and $_ -notlike 'cxx/*' -and $_ -ne 'cxx' })
        $found = foreach ($file in $files) {
            $path = Join-Path $script:Root $file
            if (Test-Path -LiteralPath $path -PathType Leaf) {
                Select-String -LiteralPath $path -SimpleMatch $script:DroppedUrl |
                    ForEach-Object { "${file}:$($_.LineNumber)" }
            }
        }
        $found | Should -BeNullOrEmpty
    }
}
