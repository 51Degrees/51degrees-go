# 51Degrees Common Go-Code Library

![51Degrees](https://raw.githubusercontent.com/51Degrees/51degrees-go/main/.github/images/logo-160.png "Data rewards the curious") **51Degrees Common C Code**

[Reference Documentation](https://pkg.go.dev/github.com/51Degrees/51degrees-go/common/v4 "Reference documentation")

## Introduction

The 51Degrees Common Go Library provides essential utilities and common functionality for 51Degrees Go projects. This library includes file management, logging, error handling, and data file update mechanisms that are shared across various 51Degrees Go implementations.

## Features

- **File Watching**: Monitor file changes and automatically trigger updates
- **File Pulling**: Download and manage data files from remote sources. A download is streamed into a temp file next to the data file and renamed over it, so the data file's directory needs free space for a second copy of the file
- **File Updating**: Handle data file updates with validation and error handling
- **Logging**: Configurable logging system for debugging and monitoring
- **Error Handling**: Standardized error types and handling mechanisms

## Installation

```bash
go get github.com/51Degrees/51degrees-go/common/v4
```

## Usage

### File Watching

The library provides file watching capabilities to monitor changes to data files:

```bash
go import "github.com/51Degrees/51degrees-go/common/v4"
// Create a file watcher watcher, err := NewFileWatcher(filePath) if err != nil { log.Fatal(err) }
// Start watching for changes go watcher.Watch()
```


### File Updating

Manage data file updates with built-in validation:

```bash
go import "github.com/51Degrees/51degrees-go/common/v4"
// Create a file updater updater := NewFileUpdater(config)
// Update data files err := updater.Update() if err != nil { log.Printf("Update failed: %v", err) }

```


### Logging

Configure logging for your application:

```go
import "github.com/51Degrees/51degrees-go/common/v4"

// Initialize logger
logger := NewLogger(logLevel)

// Use logger
logger.Info("Application started")
logger.Error("An error occurred", err)
```

## Components
### File Management
- - File system monitoring `file_watch.go`
- - Remote file downloading `file_pulling.go`
- - Data file update management `file_updater.go`
- - General file watching utilities `watcher.go`

### Utilities
- - Logging functionality `logger.go`
- - Error handling and custom error types `errors.go`

## Testing
The library includes comprehensive test suites:

```bash
go test ./...
```

Test files:
- `file_watch_test.go`
- `file_pulling_test.go`
- `file_updater_test.go`
- `logger_test.go`

## Requirements
- Go 1.19 or higher
- Compatible with Go modules

## Related Projects
- [51Degrees Device Detection Go](https://github.com/51Degrees/51degrees-go/tree/main/device-detection)
- [51Degrees IP Intelligence Go](https://github.com/51Degrees/51degrees-go/tree/main/ip-intelligence)
