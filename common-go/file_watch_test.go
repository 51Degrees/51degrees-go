package common_go

import (
	"compress/gzip"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

func unzipAndSaveToTempFile(name string) (*os.File, error) {
	mockHashMutex.Lock()
	file, err := os.Open("mock_hash.gz")
	defer mockHashMutex.Unlock()
	defer file.Close()
	if err != nil {
		return nil, err
	}
	gReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gReader.Close()

	uncompressed, err := io.ReadAll(gReader)
	err = os.WriteFile(name, uncompressed, 0644)
	if err != nil {
		return nil, err
	}

	uFile, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer uFile.Close()

	return uFile, nil
}

// WatchEvent interface for testing
type WatchEvent interface {
	Name() string
}

// Watcher interface for testing
type WatcherTest struct {
	impl WatcherImpl
}

type WatcherImpl interface {
	Changed() <-chan WatchEvent
	Errors() <-chan error
	Start() error
	Stop() error
}

// Mock implementations for testing
type mockWatcher struct {
	changedCh chan WatchEvent
	errorsCh  chan error
	isStarted bool
	isStopped bool
	startErr  error
	stopErr   error
}

func (m *mockWatcher) Changed() <-chan WatchEvent {
	return m.changedCh
}

func (m *mockWatcher) Errors() <-chan error {
	return m.errorsCh
}

func (m *mockWatcher) Start() error {
	if m.startErr != nil {
		return m.startErr
	}
	m.isStarted = true
	return nil
}

func (m *mockWatcher) Stop() error {
	if m.stopErr != nil {
		return m.stopErr
	}
	m.isStopped = true
	close(m.changedCh)
	close(m.errorsCh)
	return nil
}

func newMockWatcher() *mockWatcher {
	return &mockWatcher{
		changedCh: make(chan WatchEvent, 10),
		errorsCh:  make(chan error, 10),
	}
}

type mockWatchEvent struct {
	name string
}

func (m mockWatchEvent) Name() string {
	return m.name
}

type mockLogWrapper struct {
	messages []string
	mu       sync.Mutex
}

func (m *mockLogWrapper) Printf(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, format)
}

func (m *mockLogWrapper) getMessages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.messages...)
}

// Mock the newWatcher function for testing
var newWatcherTest = func(path string, debounce, settle time.Duration) (*Watcher, error) {
	return nil, errors.New("default implementation not available")
}

func TestNewFileWatcher_Success(t *testing.T) {
	// Setup
	originalNewWatcher := newWatcherTest
	defer func() { newWatcherTest = originalNewWatcher }()

	newWatcherTest = func(path string, debounce, settle time.Duration) (*Watcher, error) {
		if path != "/test/path" {
			t.Errorf("Expected path '/test/path', got '%s'", path)
		}
		if debounce != time.Second {
			t.Errorf("Expected debounce time.Second, got %v", debounce)
		}
		if settle != time.Second {
			t.Errorf("Expected settle time.Second, got %v", settle)
		}
		return &Watcher{}, nil
	}

	logger := &LogWrapper{}
	stopCh := make(chan *sync.WaitGroup, 1)
	path := "/test/path"

	// Execute
	fw, err := newFileWatcher(logger, path, stopCh)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if fw == nil {
		t.Fatal("Expected FileWatcher to be created")
	}
	if fw.logger != logger {
		t.Error("Expected logger to be set correctly")
	}
	if fw.stopCh != stopCh {
		t.Error("Expected stopCh to be set correctly")
	}
	if fw.watcher == nil {
		t.Error("Expected watcher to be set")
	}
	if fw.callback == nil {
		t.Error("Expected callback to be initialized")
	}

	// Verify callback is set to empty function
	fw.callback() // Should not panic
}

func TestFileWatcher_Watch_Success(t *testing.T) {
	// Setup
	logger := &LogWrapper{}
	stopCh := make(chan *sync.WaitGroup, 1)

	fw := &FileWatcher{
		watcher:  &Watcher{},
		logger:   logger,
		stopCh:   stopCh,
		callback: func() {},
	}

	callbackCalled := false
	onChange := func() {
		callbackCalled = true
	}

	// Execute
	err := fw.watch(onChange)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify callback is set correctly
	fw.callback()
	if !callbackCalled {
		t.Error("Expected callback to be set correctly")
	}
}

func TestFileWatcher_Run_MultipleEvents(t *testing.T) {
	// Setup
	mockW := newMockWatcher()
	logger := &LogWrapper{}
	stopCh := make(chan *sync.WaitGroup, 1)

	callbackCount := 0
	fw := &FileWatcher{
		watcher: &Watcher{},
		logger:  logger,
		stopCh:  stopCh,
		callback: func() {
			callbackCount++
		},
	}

	// Execute
	done := make(chan error, 1)
	go func() {
		done <- fw.run()
	}()

	// Send multiple events
	mockW.changedCh <- mockWatchEvent{name: "test1.txt"}
	mockW.changedCh <- mockWatchEvent{name: "test2.txt"}
	mockW.errorsCh <- errors.New("test error")

	// Allow time for event processing
	time.Sleep(50 * time.Millisecond)

	// Send stop signal
	wg := &sync.WaitGroup{}
	wg.Add(1)
	stopCh <- wg

	// Assert
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Expected no error from run(), got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run() did not complete within timeout")
	}
}

func TestFileWatcher_Run_NilCallback(t *testing.T) {
	// Setup
	mockW := newMockWatcher()
	logger := &LogWrapper{}
	stopCh := make(chan *sync.WaitGroup, 1)

	fw := &FileWatcher{
		watcher:  &Watcher{},
		logger:   logger,
		stopCh:   stopCh,
		callback: nil, // nil callback
	}

	// Execute
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- errors.New("panic occurred")
				return
			}
			done <- nil
		}()
		fw.run()
	}()

	// Send file change event
	mockW.changedCh <- mockWatchEvent{name: "test.txt"}

	// Allow time for event processing
	time.Sleep(10 * time.Millisecond)

	// Send stop signal
	wg := &sync.WaitGroup{}
	wg.Add(1)
	stopCh <- wg

	// Assert
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Expected no error with nil callback, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run() did not complete within timeout")
	}
}

func TestFileWatcher_Interface_Compliance(t *testing.T) {
	// Verify that FileWatcher implements fileWatcher interface
	var _ fileWatcher = (*FileWatcher)(nil)
}

func TestFileWatcher_Run_ConcurrentAccess(t *testing.T) {
	// Setup
	mockW := newMockWatcher()
	logger := &LogWrapper{}
	stopCh := make(chan *sync.WaitGroup, 1)

	callbackCount := 0
	var mu sync.Mutex

	fw := &FileWatcher{
		watcher: &Watcher{},
		logger:  logger,
		stopCh:  stopCh,
		callback: func() {
			mu.Lock()
			callbackCount++
			mu.Unlock()
		},
	}

	// Execute
	done := make(chan error, 1)
	go func() {
		done <- fw.run()
	}()

	// Send multiple events concurrently
	go func() {
		for i := 0; i < 5; i++ {
			mockW.changedCh <- mockWatchEvent{name: "concurrent_test.txt"}
			time.Sleep(time.Millisecond)
		}
	}()

	// Allow events to be processed
	time.Sleep(50 * time.Millisecond)

	// Send stop signal
	wg := &sync.WaitGroup{}
	wg.Add(1)
	stopCh <- wg

	// Assert
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Expected no error from run(), got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run() did not complete within timeout")
	}

}
