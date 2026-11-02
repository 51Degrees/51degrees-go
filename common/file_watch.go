package common_go

import (
	"sync"
	"time"
)

// fileWatcher defines an interface for monitoring file changes and managing watcher lifecycle operations.
type fileWatcher interface {
	watch(onChange func()) error
	stop() error
	run() error
}

// FileWatcher monitors file changes and triggers a callback when a modification is detected. It manages its lifecycle.
type FileWatcher struct {
	watcher  *Watcher
	logger   *LogWrapper
	callback func()
	stopCh   chan *sync.WaitGroup
}

// run continuously monitors file changes or errors, processes events, and terminates when a stop signal is received.
func (f *FileWatcher) run() error {
	for {
		select {
		case wg := <-f.stopCh:
			f.stop()
			defer wg.Done()
			return nil
		case event := <-f.watcher.Changed():
			f.logger.Printf("File %s has been modified", event.Name())
			f.callback()
		case err, ok := <-f.watcher.Errors():
			if !ok {
				return err
			}
			if err != nil {
				f.logger.Printf("Error watching file: %v", err)
			}
		}
	}
}

// watch sets a callback function to be triggered on file changes and starts the file watcher. Returns an error if any occurs.
func (f *FileWatcher) watch(onChange func()) error {
	f.callback = onChange
	return f.watcher.Start()
}

// stop terminates the associated file watcher and stops monitoring file changes. Returns an error if the watcher is not running.
func (f *FileWatcher) stop() error {
	return f.watcher.Stop()
}

// newFileWatcher creates and initializes a new FileWatcher instance for monitoring file changes at the specified path.
func newFileWatcher(logger *LogWrapper, path string, stopCh chan *sync.WaitGroup) (*FileWatcher, error) {
	watcher, err := newWatcher(path, time.Second, time.Second)
	if err != nil {
		return nil, err
	}

	return &FileWatcher{
		watcher:  watcher,
		logger:   logger,
		callback: func() {},
		stopCh:   stopCh,
	}, nil
}
