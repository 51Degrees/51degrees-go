package common_go

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileUpdater manages to update a data file by pulling changes and optionally creating temporary copies or syncing states.
// It supports auto-updates, file change detection, and customizable intervals for data pulls with optional randomization.
type FileUpdater struct {
	dataFile     string
	dataFileUrl  string
	tempDataDir  string
	tempDataFile string

	fileWatcherStarted          bool
	isFileWatcherEnabled        bool
	isAutoUpdateEnabled         bool
	filePullerStarted           bool
	isCreateTempDataCopyEnabled bool
	isUpdateOnStartEnabled      bool
	fileSynced                  bool

	dataFilePullEveryMs        int
	totalFilePulls             int
	fileExternallyChangedCount int
	randomization              int

	lastModificationTimestamp *time.Time

	fileWatcher fileWatcher
	logger      *LogWrapper
}

// NewFileUpdater initializes a new FileUpdater instance with default settings for updating and managing a data file.
func NewFileUpdater(dataFileUrl string) *FileUpdater {
	return &FileUpdater{
		dataFileUrl: dataFileUrl,
		tempDataDir: "",

		isFileWatcherEnabled:        true,
		isAutoUpdateEnabled:         true,
		isCreateTempDataCopyEnabled: true,
		isUpdateOnStartEnabled:      false,
		fileSynced:                  false,

		dataFilePullEveryMs: 30 * 60 * 1000, // default 30 minutes
		randomization:       10 * 60 * 1000, // default 10 minutes

		logger: &LogWrapper{
			logger:  DefaultLogger,
			enabled: true,
		},
	}
}

// SetDataFileUrl sets the URL of the data file used for updates.
func (e *FileUpdater) SetDataFileUrl(dataFileUrl string) {
	e.dataFileUrl = dataFileUrl
}

// SetDataFile sets the local file path of the data file used for updates.
func (e *FileUpdater) SetDataFile(dataFileUrl string) {
	e.dataFile = dataFileUrl
}

// SetDataFilePullEveryMs sets the interval in milliseconds for pulling the data file updates.
func (e *FileUpdater) SetDataFilePullEveryMs(dataFilePullEveryMs int) {
	e.dataFilePullEveryMs = dataFilePullEveryMs
}

// SetFileWatcherStarted sets the state of the file watcher as started or stopped.
func (e *FileUpdater) SetFileWatcherStarted(fileWatcherStarted bool) {
	e.fileWatcherStarted = fileWatcherStarted
}

// SetTempDataDir sets the directory path used for storing temporary data files.
func (e *FileUpdater) SetTempDataDir(tempDataDir string) {
	e.tempDataDir = tempDataDir
}

// SetIsAutoUpdateEnabled enables or disables the auto-update behavior for the FileUpdater instance.
func (e *FileUpdater) SetIsAutoUpdateEnabled(autoUpdateEnabled bool) {
	e.isAutoUpdateEnabled = autoUpdateEnabled
}

// SetFilePullerStarted sets the state of the file puller as started or stopped.
func (e *FileUpdater) SetFilePullerStarted(filePullerStarted bool) {
	e.filePullerStarted = filePullerStarted
}

// SetLastModificationTimestamp updates the timestamp of the last modification for the data file.
func (e *FileUpdater) SetLastModificationTimestamp(lastModificationTimestamp *time.Time) {
	e.lastModificationTimestamp = lastModificationTimestamp
}

// SetIsCreateTempDataCopyEnabled enables or disables creating a temporary copy of the data file during updates.
func (e *FileUpdater) SetIsCreateTempDataCopyEnabled(createTempDataCopyEnabled bool) {
	e.isCreateTempDataCopyEnabled = createTempDataCopyEnabled
}

// SetUpdateOnStartEnabled enables or disables the update-on-start behavior for the FileUpdater instance.
func (e *FileUpdater) SetUpdateOnStartEnabled(isUpdateOnStartEnabled bool) {
	e.isUpdateOnStartEnabled = isUpdateOnStartEnabled
}

// SetRandomization sets the randomization interval in milliseconds to be added to the file pull schedule.
func (e *FileUpdater) SetRandomization(randomization int) {
	e.randomization = randomization
}

// SetLoggerEnabled enables or disables the logger for the FileUpdater instance and returns the updated LogWrapper.
func (e *FileUpdater) SetLoggerEnabled(enabled bool) *LogWrapper {
	e.logger.enabled = enabled

	return e.logger
}

// SetCustomLogger sets a custom logger for the FileUpdater instance and returns the wrapped logger configuration.
func (e *FileUpdater) SetCustomLogger(logger LogWriter) *LogWrapper {
	e.logger = &LogWrapper{
		enabled: true,
		logger:  logger,
	}

	return e.logger
}

// SetIsFileWatcherEnabled enables or disables the file watcher functionality for the FileUpdater instance.
func (e *FileUpdater) SetIsFileWatcherEnabled(enabled bool) {
	e.isFileWatcherEnabled = enabled
}

// GetLogger returns the logger instance associated with the FileUpdater.
func (e *FileUpdater) GetLogger() *LogWrapper {
	return e.logger
}

// GetDataFileUrl returns the URL of the data file associated with the FileUpdater instance.
func (e FileUpdater) GetDataFileUrl() string {
	return e.dataFileUrl
}

// GetDataFilePullEveryMs returns the interval in milliseconds between data file pull attempts.
func (e FileUpdater) GetDataFilePullEveryMs() int {
	return e.dataFilePullEveryMs
}

// GetDataFile returns the file path of the data file associated with the FileUpdater instance.
func (e FileUpdater) GetDataFile() string {
	return e.dataFile
}

// GetReloadFilePath returns the file path to be used for reloading, optionally creating a temporary copy if required.
func (e *FileUpdater) GetReloadFilePath() (string, error) {
	var err error
	reloadFilePath := e.GetDataFile()

	if e.isCreateTempDataCopyEnabled {
		reloadFilePath, err = e.copyFileForReloadManager()
		if err != nil {
			return "", err
		}
	}

	return reloadFilePath, nil
}

// IsDataFileProvided checks if a data file is specified by verifying whether GetDataFile returns a non-empty string.
func (e FileUpdater) IsDataFileProvided() bool {
	return e.GetDataFile() != ""
}

// IsAutoUpdateEnabled checks if the auto-update feature is enabled for the FileUpdater instance.
func (e FileUpdater) IsAutoUpdateEnabled() bool {
	return e.isAutoUpdateEnabled
}

// IsFilePullerStarted checks if the file puller has been started and returns a boolean value indicating its status.
func (e FileUpdater) IsFilePullerStarted() bool {
	return e.filePullerStarted
}

// IsCreateTempDataCopyEnabled checks if the creation of a temporary data copy is enabled for the FileUpdater instance.
func (e FileUpdater) IsCreateTempDataCopyEnabled() bool {
	return e.isCreateTempDataCopyEnabled
}

// IsUpdateOnStartEnabled checks if the update process is enabled to run automatically at the start.
func (e FileUpdater) IsUpdateOnStartEnabled() bool {
	return e.isUpdateOnStartEnabled
}

// IsFileWatcherStarted checks if the file watcher service has been initiated and is currently running.
func (e FileUpdater) IsFileWatcherStarted() bool {
	return e.fileWatcherStarted
}

// IsFileWatcherEnabled checks if the file watcher feature is enabled for the FileUpdater instance.
func (e FileUpdater) IsFileWatcherEnabled() bool {
	return e.isFileWatcherEnabled
}

// copyFileForReloadManager copies a file to a temporary location and returns its path for the reload manager to use.
func (e *FileUpdater) copyFileForReloadManager() (string, error) {
	dirPath, tempFilepath, err := e.copyToTempFile()
	if err != nil {
		return "", err
	}
	e.tempDataFile = tempFilepath
	fullPath := filepath.Join(dirPath, tempFilepath)

	return fullPath, nil
}

// copyToTempFile copies the contents of the data file to a temporary file and returns the temp directory and file name.
func (e *FileUpdater) copyToTempFile() (string, string, error) {
	src, err := os.Open(e.dataFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to read data file: %w", err)
	}
	defer src.Close()
	originalFileName := filepath.Base(e.dataFile)

	f, err := os.CreateTemp(e.tempDataDir, originalFileName)
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp data file: %w", err)
	}
	defer f.Close()

	// streamed so that a multi-gigabyte data file is not loaded into memory
	_, err = io.Copy(f, src)
	if err != nil {
		return "", "", fmt.Errorf("failed to write temp data file: %w", err)
	}

	tempFileName := filepath.Base(f.Name())

	return e.tempDataDir, tempFileName, nil
}

// InitFileWatcher initializes the file watcher to monitor changes in the data file and set up a callback for file updates.
func (e *FileUpdater) InitFileWatcher(logger *LogWrapper, stopCh chan *sync.WaitGroup) error {
	watcher, err := newWatcher(e.dataFile, time.Second, time.Second)
	if err != nil {
		return err
	}

	e.fileWatcher = &FileWatcher{
		watcher:  watcher,
		logger:   logger,
		callback: func() {},
		stopCh:   stopCh,
	}

	return nil
}

// Watch monitors file changes and triggers the provided onChange callback function when a change is detected.
func (e *FileUpdater) Watch(onChange func()) error {
	return e.fileWatcher.watch(onChange)
}

// RunWatcher starts the file watcher's goroutine to monitor file changes asynchronously.
func (e *FileUpdater) RunWatcher() {
	go e.fileWatcher.run()
}

// getFilePath returns the file path of the data file or its temporary copy depending on configuration settings.
func (e *FileUpdater) getFilePath() string {
	if e.isCreateTempDataCopyEnabled {
		return filepath.Join(e.tempDataDir, e.tempDataFile)
	}

	return e.dataFile
}

// IncreaseFileExternallyChangedCount increments the counter tracking the number of times a file has been externally modified.
func (e *FileUpdater) IncreaseFileExternallyChangedCount() {
	e.fileExternallyChangedCount++
}

// InitCreateTempDataCopy initializes and creates a temporary data directory if enabled and not already set.
func (e *FileUpdater) InitCreateTempDataCopy() error {
	if e.isCreateTempDataCopyEnabled && e.tempDataDir == "" {
		path, err := os.MkdirTemp("", "51degrees-on-premise")
		if err != nil {
			return err
		}

		e.tempDataDir = path
	}

	return nil
}
