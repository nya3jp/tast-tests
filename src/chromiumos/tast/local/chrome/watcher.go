// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
)

const (
	checkBrowserInterval = 100 * time.Millisecond // interval to check browser process
)

// browserWatcher watches the browser process to attempt to identify situations where Chrome is crashing.
type browserWatcher struct {
	rootProcess *process.Process // The handle for the browser process.
	browserErr  error            // error that was detected, if any
	mutex       sync.Mutex       // protects browserErr
	done        chan bool        // used to tell the watcher's goroutine to exit
	closed      chan error       // used to wait for the goroutine to exit
}

// newBrowserWatcher creates a new browserWatcher and starts it.
func newBrowserWatcher() *browserWatcher {
	bw := &browserWatcher{done: make(chan bool, 1), closed: make(chan error, 1)}
	go func() {
		defer func() {
			bw.closed <- bw.err()
		}()
		bw.check()
		for {
			select {
			case <-bw.done:
				return
			case <-time.After(checkBrowserInterval):
				if !bw.check() {
					return
				}
			}
		}
	}()
	return bw
}

// close synchronously stops the watch goroutine.
func (bw *browserWatcher) close() error {
	bw.done <- true
	return <-bw.closed
}

// err returns the first error that was observed or nil if no error was observed.
func (bw *browserWatcher) err() error {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()
	return bw.browserErr
}

// check is an internal method that checks the browser process, updating rootProcess and browserErr as needed.
// Returns false after an error has been encountered, indicating that no further calls are needed.
func (bw *browserWatcher) check() bool {
	if bw.rootProcess != nil {
		isRunning, err := bw.rootProcess.IsRunning()
		bw.mutex.Lock()
		defer bw.mutex.Unlock()
		if err != nil {
			bw.browserErr = errors.Wrapf(err, "browser process %d exited; Chrome probably crashed", bw.rootProcess.Pid)
			return false
		}
		if !isRunning {
			bw.browserErr = errors.Errorf("browser process %d exited; Chrome probably crashed", bw.rootProcess.Pid)
			return false
		}
		// Theoretically, there's a chance that the browser process has finished
		// and another process with the same PID is running, though it would be
		// rare.
		return true
	}

	pid, err := GetRootPID()
	if err != nil {
		// The browser process might not have started yet. Just keep checking.
		return true
	}

	proc, err := process.NewProcess(int32(pid))

	if err != nil {
		bw.mutex.Lock()
		defer bw.mutex.Unlock()
		bw.browserErr = errors.Errorf("browser process %d exited; Chrome probably crashed", pid)
		return false
	}
	bw.rootProcess = proc
	return true
}
