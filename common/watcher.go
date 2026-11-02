package common_go

import (
	"fmt"
	"os"
	"time"
)

// Watcher represents a file monitoring mechanism that detects changes to a specified file at regular intervals.
type Watcher struct {
	filepath  string
	interval  time.Duration
	wait      time.Duration
	changed   chan os.FileInfo
	errors    chan error
	stopInter chan bool
	stop      chan bool
	running   bool
}

// newWatcher creates and initializes a Watcher for monitoring a specified file, interval, and wait durations.
func newWatcher(filepath string, interval time.Duration, wait time.Duration) (*Watcher, error) {
	info, err := os.Stat(filepath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if !os.IsNotExist(err) && info.IsDir() {
		return nil, fmt.Errorf("error: the path '%s' is a directory", filepath)
	}
	if interval < time.Microsecond {
		return nil, fmt.Errorf("error: the interval is less than 1 μs")
	}
	if wait < time.Microsecond {
		return nil, fmt.Errorf("error: the wait is less than 1 μs")
	}
	return &Watcher{
		filepath:  filepath,
		interval:  interval,
		wait:      wait,
		changed:   make(chan os.FileInfo),
		errors:    make(chan error),
		stopInter: make(chan bool),
		stop:      make(chan bool),
		running:   false,
	}, nil
}

// Start initializes and begins monitoring the specified file for changes at regular intervals. It runs asynchronously.
// Returns an error if the watcher is already running.
func (fw *Watcher) Start() error {
	if fw.running {
		return fmt.Errorf("error: the watcher is already running")
	}
	fw.running = true

	go func() {
		hasChanged := false
		prevInfo, _ := os.Stat(fw.filepath)

		for {
			select {
			case <-fw.stopInter:
				fw.running = false
				fw.stop <- true
				return
			default:
				info, err := os.Stat(fw.filepath)
				if err != nil {
					if !os.IsNotExist(err) {
						fw.errors <- err
					}
					time.Sleep(fw.interval)
					continue
				}

				if prevInfo == nil ||
					info.Size() != prevInfo.Size() ||
					info.ModTime() != prevInfo.ModTime() {

					hasChanged = true
					prevInfo = info

					time.Sleep(fw.wait)
					continue

				} else if hasChanged {
					hasChanged = false
					fw.changed <- info
				}

				time.Sleep(fw.interval)
			}
		}
	}()

	return nil
}

// Changed returns a channel that emits os.FileInfo for each detected file modification.
func (fw *Watcher) Changed() <-chan os.FileInfo {
	return fw.changed
}

// Errors returns a channel that outputs errors encountered during file monitoring.
func (fw *Watcher) Errors() <-chan error {
	return fw.errors
}

// IsStopped returns a channel that signals when the watcher has stopped running.
func (fw *Watcher) IsStopped() <-chan bool {
	return fw.stop
}

// Stop terminates the watcher and stops monitoring file changes. Returns an error if the watcher is not running.
func (fw *Watcher) Stop() error {
	if !fw.running {
		return fmt.Errorf("error: the watcher is not running")
	}
	fw.stopInter <- true
	return nil
}
