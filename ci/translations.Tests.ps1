# Tests for ci/update-translations.ps1, with Pester 5.
#
#   Invoke-Pester ci/translations.Tests.ps1
#
# ip-intelligence/translation embeds a copy of the IP Intelligence translation
# data, which the C library keeps in its data submodule. Each test lays out a
# throwaway repository with the script, the C library's copy and the
# package's copy, then runs the script in it.

BeforeAll {
    $script:Script = Join-Path $PSScriptRoot 'update-translations.ps1'

    # Where the C library keeps the translation data.
    $script:Source = 'cxx/data/ip-intelligence/Translations'
    # Where the package embeds its copy.
    $script:Target = 'ip-intelligence/translation/Translations'

    $script:Files = [ordered]@{
        'countrycodes.en_GB.yml'     = "FR: France`nGB: United Kingdom`n"
        'OSM/countries.fr_FR.yml'    = "United Kingdom: Royaume-Uni`n"
        'OSM/countries.de_DE.yml'    = "United Kingdom: Vereinigtes Koenigreich`n"
    }

    function Write-Tree([string]$Root, [Collections.IDictionary]$Files) {
        foreach ($name in $Files.Keys) {
            $full = Join-Path $Root $name
            $null = New-Item -ItemType Directory -Force (Split-Path $full)
            [IO.File]::WriteAllText($full, $Files[$name])
        }
    }

    function New-Repository([switch]$WithCopy) {
        $repo = Join-Path $TestDrive ([Guid]::NewGuid().ToString('N'))
        $null = New-Item -ItemType Directory -Force (Join-Path $repo 'ci')
        Copy-Item $script:Script (Join-Path $repo 'ci')
        Write-Tree (Join-Path $repo $script:Source) $script:Files
        if ($WithCopy) {
            Write-Tree (Join-Path $repo $script:Target) $script:Files
        }
        return $repo
    }

    function Invoke-Update([string]$Repo, [switch]$Check) {
        & (Join-Path $Repo 'ci/update-translations.ps1') -Check:$Check
    }
}

Describe 'update-translations.ps1' {
    It 'copies the C library''s translation data into the package' {
        $repo = New-Repository
        Invoke-Update $repo
        foreach ($name in $script:Files.Keys) {
            $copy = Join-Path $repo $script:Target $name
            [IO.File]::ReadAllText($copy) | Should -BeExactly $script:Files[$name]
        }
    }

    It 'passes -Check when the copy matches' {
        $repo = New-Repository -WithCopy
        { Invoke-Update $repo -Check } | Should -Not -Throw
    }

    It 'passes -Check when only the line endings differ' {
        $repo = New-Repository -WithCopy
        $copy = Join-Path $repo $script:Target 'countrycodes.en_GB.yml'
        $crlf = $script:Files['countrycodes.en_GB.yml'] -replace "`n", "`r`n"
        [IO.File]::WriteAllText($copy, $crlf)
        { Invoke-Update $repo -Check } | Should -Not -Throw
    }

    It 'fails -Check when a file differs' {
        $repo = New-Repository -WithCopy
        $copy = Join-Path $repo $script:Target 'OSM/countries.fr_FR.yml'
        [IO.File]::WriteAllText($copy, "United Kingdom: Angleterre`n")
        { Invoke-Update $repo -Check } |
            Should -Throw '*OSM/countries.fr_FR.yml*'
    }

    It 'fails -Check when a file is missing from the copy' {
        $repo = New-Repository -WithCopy
        Remove-Item (Join-Path $repo $script:Target 'OSM/countries.de_DE.yml')
        { Invoke-Update $repo -Check } |
            Should -Throw '*OSM/countries.de_DE.yml*'
    }

    It 'fails -Check when the copy has a file the C library does not' {
        $repo = New-Repository -WithCopy
        Write-Tree (Join-Path $repo $script:Target) @{
            'OSM/countries.xx_XX.yml' = "France: France`n" }
        { Invoke-Update $repo -Check } |
            Should -Throw '*OSM/countries.xx_XX.yml*'
    }

    It 'fails when the C library''s translation data is not there' {
        $repo = New-Repository -WithCopy
        Remove-Item -Recurse (Join-Path $repo $script:Source)
        { Invoke-Update $repo -Check } | Should -Throw '*was not found*'
    }
}
