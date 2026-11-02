# 51Degrees Common Go-Code Library

![51Degrees](https://51degrees.com/img/logo.png?utm_source=github&utm_medium=readme&utm_campaign=common-go&utm_content=readme.md&utm_term=51degrees-common-go-code-library "Data rewards the curious") **51Degrees Common C Code**

[Reference Documentation](https://51degrees.github.io/ip-intelligence-go/4.4/examples.html "Reference documentation")

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
go get github.com/51Degrees/common-go/v4
```

## Usage

### File Watching

The library provides file watching capabilities to monitor changes to data files:

```bash
go import "github.com/51Degrees/common-go/v4"
// Create a file watcher watcher, err := NewFileWatcher(filePath) if err != nil { log.Fatal(err) }
// Start watching for changes go watcher.Watch()
```


### File Updating

Manage data file updates with built-in validation:

```bash
go import "github.com/51Degrees/common-go/v4"
// Create a file updater updater := NewFileUpdater(config)
// Update data files err := updater.Update() if err != nil { log.Printf("Update failed: %v", err) }

```


### Logging

Configure logging for your application:

```go
import "github.com/51Degrees/common-go/v4"

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
- [51Degrees Device Detection Go](https://github.com/51Degrees/device-detection-go)
- [51Degrees IP Intelligence Go](https://github.com/51Degrees/ip-intelligence-go)
