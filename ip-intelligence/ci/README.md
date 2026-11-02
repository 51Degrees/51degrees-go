# IP Intelligence Go - CI Directory

This directory contains the CI/CD scripts and configuration for the ip-intelligence-go library.

## Amalgamation Process

The IP Intelligence Go library uses an amalgamation process to create a single-file C/C++ source distribution that can be compiled with CGO. This process combines multiple source files from the `ip-intelligence-cxx` repository into two files:

- `ipi_interop/ip-intelligence-cxx.h` - Combined header file
- `ipi_interop/ip-intelligence-cxx.c` - Combined source file

### How Amalgamation Works

The amalgamation is handled by the `update-packages.ps1` script, which:

1. **Updates Go dependencies:**
   ```powershell
   go get -u ./...
   go mod tidy
   ```

2. **Clones the latest ip-intelligence-cxx repository:**
   ```powershell
   git clone --depth=1 --recurse-submodules --shallow-submodules "https://github.com/51Degrees/ip-intelligence-cxx.git"
   ```

3. **Generates amalgamated header file:**
   ```powershell
   awk -f ci/amalgamate.awk $src/fiftyone.h $src/ip-graph-cxx/graph.h >ipi_interop/ip-intelligence-cxx.h
   ```

4. **Generates amalgamated source file:**
   ```powershell
   awk -f ci/amalgamate.awk $src/common-cxx/*.c $src/ip-graph-cxx/*.c $src/*.c >ipi_interop/ip-intelligence-cxx.c
   ```

5. **Cleans up temporary files:**
   ```powershell
   Remove-Item -Recurse -Force ip-intelligence-cxx
   ```

### Source Components

The amalgamation combines files from several sources:

**Header Files:**
- `ip-intelligence-cxx/src/fiftyone.h` - Main IP Intelligence header
- `ip-intelligence-cxx/src/ip-graph-cxx/graph.h` - Graph functionality header

**Source Files:**
- `ip-intelligence-cxx/src/common-cxx/*.c` - Common 51Degrees C library code
- `ip-intelligence-cxx/src/ip-graph-cxx/*.c` - Graph functionality C code
- `ip-intelligence-cxx/src/*.c` - IP Intelligence specific C code

### Amalgamation Script

The `amalgamate.awk` script processes C/C++ source files by:
- Recursively following `#include "..."` directives (but not system includes with `<>`)
- Tracking included files to prevent duplicate inclusions
- Normalizing header paths and handling relative includes with `../`
- Outputting a single combined source file

### Running Amalgamation

#### Automated (Recommended)
The amalgamation runs automatically via GitHub Actions:
- **Manual trigger:** Use the "Package Update" workflow in GitHub Actions
- **Nightly updates:** Can be scheduled in the workflow configuration

#### Manual Execution
```powershell
# From the ip-intelligence-go root directory
.\ci\update-packages.ps1 -RepoName "ip-intelligence-go" -OrgName "51Degrees"
```

### Dependencies

The amalgamation process requires:
- **AWK:** For running the `amalgamate.awk` script
- **Git:** For cloning source repositories
- **PowerShell:** For script execution
- **Network Access:** To clone repositories from GitHub
- **Go:** For updating dependencies

### Workflow Integration

The amalgamation is integrated into the CI/CD pipeline through:

1. **GitHub Actions Workflow:** `.github/workflows/nightly-package-update.yml`
2. **Common CI Script:** `common-ci/nightly-package-update.ps1` 
3. **Local Update Script:** `ci/update-packages.ps1` (this directory)

When changes are detected, the system automatically:
- Commits the updated amalgamated files
- Creates a pull request for review
- Triggers build and test workflows

### Troubleshooting

**Common Issues:**
- **AWK not found:** Ensure AWK is installed and available in PATH
- **Git clone fails:** Check network connectivity and repository access
- **Permission errors:** Ensure proper write permissions in the repository directory

**Manual Recovery:**
If the automated process fails, you can manually run the amalgamation script and commit the results.