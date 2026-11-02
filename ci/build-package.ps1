Write-Output "Each module is published by its tag, so there is nothing to build"
# The shared workflow requires something in the package directory
New-Item package/dummy.txt -Force | Out-Null
