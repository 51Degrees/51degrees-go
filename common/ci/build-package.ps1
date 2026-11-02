Write-Output "The repo is the package, no need for special packaging"
# CI workflow requires something to be present in the 'package' directory
New-Item package/dummy.txt -Force || $(throw "failed to create dummy.txt")
