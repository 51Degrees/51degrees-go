package common_go

import (
	"testing"
	"time"
)

// Mock implementations for testing
type mockLogWriter struct {
	messages []string
}

func (m *mockLogWriter) Printf(format string, args ...interface{}) {
	// Simple mock implementation
}

type mockFileWatcher struct {
	watchCalled bool
	runCalled   bool
}

func (m *mockFileWatcher) watch(onChange func()) error {
	m.watchCalled = true
	return nil
}

func (m *mockFileWatcher) run() {
	m.runCalled = true
}

func TestNewFileUpdater(t *testing.T) {
	dataFileUrl := "http://example.com/data.ipi"
	updater := NewFileUpdater(dataFileUrl)

	if updater.dataFileUrl != dataFileUrl {
		t.Errorf("Expected dataFileUrl %s, got %s", dataFileUrl, updater.dataFileUrl)
	}

	if updater.tempDataDir != "" {
		t.Errorf("Expected empty tempDataDir, got %s", updater.tempDataDir)
	}

	if !updater.isFileWatcherEnabled {
		t.Error("Expected isFileWatcherEnabled to be true")
	}

	if !updater.isAutoUpdateEnabled {
		t.Error("Expected isAutoUpdateEnabled to be true")
	}

	if !updater.isCreateTempDataCopyEnabled {
		t.Error("Expected isCreateTempDataCopyEnabled to be true")
	}

	if updater.isUpdateOnStartEnabled {
		t.Error("Expected isUpdateOnStartEnabled to be false")
	}

	expectedInterval := 30 * 60 * 1000
	if updater.dataFilePullEveryMs != expectedInterval {
		t.Errorf("Expected dataFilePullEveryMs %d, got %d", expectedInterval, updater.dataFilePullEveryMs)
	}

	expectedRandomization := 10 * 60 * 1000
	if updater.randomization != expectedRandomization {
		t.Errorf("Expected randomization %d, got %d", expectedRandomization, updater.randomization)
	}
}

func TestFileUpdater_SettersAndGetters(t *testing.T) {
	updater := NewFileUpdater("http://example.com/data.ipi")

	// Test SetDataFileUrl and GetDataFileUrl
	newUrl := "http://newurl.com/data.ipi"
	updater.SetDataFileUrl(newUrl)
	if updater.GetDataFileUrl() != newUrl {
		t.Errorf("Expected dataFileUrl %s, got %s", newUrl, updater.GetDataFileUrl())
	}

	// Test SetDataFile and GetDataFile
	dataFile := "/path/to/data.ipi"
	updater.SetDataFile(dataFile)
	if updater.GetDataFile() != dataFile {
		t.Errorf("Expected dataFile %s, got %s", dataFile, updater.GetDataFile())
	}

	// Test SetDataFilePullEveryMs and GetDataFilePullEveryMs
	interval := 60000
	updater.SetDataFilePullEveryMs(interval)
	if updater.GetDataFilePullEveryMs() != interval {
		t.Errorf("Expected interval %d, got %d", interval, updater.GetDataFilePullEveryMs())
	}

	// Test SetTempDataDir
	tempDir := "/tmp/test"
	updater.SetTempDataDir(tempDir)
	if updater.tempDataDir != tempDir {
		t.Errorf("Expected tempDataDir %s, got %s", tempDir, updater.tempDataDir)
	}

	// Test SetRandomization
	randomization := 5000
	updater.SetRandomization(randomization)
	if updater.randomization != randomization {
		t.Errorf("Expected randomization %d, got %d", randomization, updater.randomization)
	}
}

func TestFileUpdater_BooleanSettersAndGetters(t *testing.T) {
	updater := NewFileUpdater("http://example.com/data.ipi")

	// Test SetIsAutoUpdateEnabled and IsAutoUpdateEnabled
	updater.SetIsAutoUpdateEnabled(false)
	if updater.IsAutoUpdateEnabled() {
		t.Error("Expected IsAutoUpdateEnabled to be false")
	}

	// Test SetFileWatcherStarted and IsFileWatcherStarted
	updater.SetFileWatcherStarted(true)
	if !updater.IsFileWatcherStarted() {
		t.Error("Expected IsFileWatcherStarted to be true")
	}

	// Test SetFilePullerStarted and IsFilePullerStarted
	updater.SetFilePullerStarted(true)
	if !updater.IsFilePullerStarted() {
		t.Error("Expected IsFilePullerStarted to be true")
	}

	// Test SetIsCreateTempDataCopyEnabled and IsCreateTempDataCopyEnabled
	updater.SetIsCreateTempDataCopyEnabled(false)
	if updater.IsCreateTempDataCopyEnabled() {
		t.Error("Expected IsCreateTempDataCopyEnabled to be false")
	}

	// Test SetUpdateOnStartEnabled and IsUpdateOnStartEnabled
	updater.SetUpdateOnStartEnabled(true)
	if !updater.IsUpdateOnStartEnabled() {
		t.Error("Expected IsUpdateOnStartEnabled to be true")
	}

	// Test SetIsFileWatcherEnabled and IsFileWatcherEnabled
	updater.SetIsFileWatcherEnabled(false)
	if updater.IsFileWatcherEnabled() {
		t.Error("Expected IsFileWatcherEnabled to be false")
	}
}

func TestFileUpdater_IsDataFileProvided(t *testing.T) {
	updater := NewFileUpdater("http://example.com/data.ipi")

	// Initially no data file is set
	if updater.IsDataFileProvided() {
		t.Error("Expected IsDataFileProvided to be false when no data file is set")
	}

	// Set a data file
	updater.SetDataFile("/path/to/data.ipi")
	if !updater.IsDataFileProvided() {
		t.Error("Expected IsDataFileProvided to be true when data file is set")
	}
}

func TestFileUpdater_LoggerMethods(t *testing.T) {
	updater := NewFileUpdater("http://example.com/data.ipi")

	// Test GetLogger
	logger := updater.GetLogger()
	if logger == nil {
		t.Error("Expected GetLogger to return non-nil logger")
	}

	// Test SetLoggerEnabled
	returnedLogger := updater.SetLoggerEnabled(false)
	if returnedLogger.enabled {
		t.Error("Expected logger to be disabled")
	}

	// Test SetCustomLogger
	mockLogger := &mockLogWriter{}
	returnedLogger = updater.SetCustomLogger(mockLogger)
	if !returnedLogger.enabled {
		t.Error("Expected custom logger to be enabled")
	}
	if returnedLogger.logger != mockLogger {
		t.Error("Expected custom logger to be set")
	}
}

func TestFileUpdater_TimestampMethods(t *testing.T) {
	updater := NewFileUpdater("http://example.com/data.ipi")

	// Test SetLastModificationTimestamp
	now := time.Now()
	updater.SetLastModificationTimestamp(&now)
	if updater.lastModificationTimestamp != &now {
		t.Error("Expected lastModificationTimestamp to be set")
	}
}

func TestFileUpdater_IncreaseFileExternallyChangedCount(t *testing.T) {
	updater := NewFileUpdater("http://example.com/data.ipi")

	initialCount := updater.fileExternallyChangedCount
	updater.IncreaseFileExternallyChangedCount()
	if updater.fileExternallyChangedCount != initialCount+1 {
		t.Errorf("Expected fileExternallyChangedCount to be %d, got %d", initialCount+1, updater.fileExternallyChangedCount)
	}
}

func TestFileUpdater_InitCreateTempDataCopy(t *testing.T) {
	updater := NewFileUpdater("http://example.com/data.ipi")

	// Test when isCreateTempDataCopyEnabled is true and tempDataDir is empty
	updater.SetIsCreateTempDataCopyEnabled(true)
	err := updater.InitCreateTempDataCopy()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if updater.tempDataDir == "" {
		t.Error("Expected tempDataDir to be set")
	}

	// Test when isCreateTempDataCopyEnabled is false
	updater2 := NewFileUpdater("http://example.com/data.ipi")
	updater2.SetIsCreateTempDataCopyEnabled(false)
	err = updater2.InitCreateTempDataCopy()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if updater2.tempDataDir != "" {
		t.Error("Expected tempDataDir to remain empty")
	}

	// Test when tempDataDir is already set
	updater3 := NewFileUpdater("http://example.com/data.ipi")
	updater3.SetTempDataDir("/existing/path")
	oldTempDir := updater3.tempDataDir
	err = updater3.InitCreateTempDataCopy()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if updater3.tempDataDir != oldTempDir {
		t.Error("Expected tempDataDir to remain unchanged")
	}
}
