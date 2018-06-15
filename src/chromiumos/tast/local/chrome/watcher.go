// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"fmt"
	"sync"
	"time"
)

const (
	checkBrowserInterval = 100 * time.Millisecond // interval to check browser process
)

// browserWatcher watches the browser process to attempt to identify situations where Chrome is crashing.
type browserWatcher struct {
	initialPID int        // first browser PID that was seen; initially -1
	browserErr error      // error that was detected, if any
	mutex      sync.Mutex // protects initialPID and browserErr
	done       chan bool  // used to tell the watcher's goroutine to exit
}

func newBrowserWatcher() *browserWatcher {
	return &browserWatcher{initialPID: -1, done: make(chan bool, 1)}
}

func (bw *browserWatcher) close() {
	bw.done <- true
}

// start begins asynchronously watching the browser process.
func (bw *browserWatcher) start() {
	go func() {
		bw.check()
		for {
			select {
			case <-bw.done:
				break
			case <-time.After(checkBrowserInterval):
				if !bw.check() {
					break
				}
			}
		}
	}()
}

// err returns the first error that was observed or nil if no error was observed.
func (bw *browserWatcher) err() error {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()
	return bw.browserErr
}

// check is an internal method that checks the browser process, updating initialPID and browserErr as needed.
// Returns false after an error has been encountered, indicating that no further calls are needed.
func (bw *browserWatcher) check() bool {
	pid, err := GetRootPID()
	if err != nil {
		pid = -1
	}

	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	// TODO(derat): Consider also checking for crash reports or log messages.

	// If Chrome hadn't previously started (and possibly still hasn't started), keep checking.
	if bw.initialPID == -1 {
		bw.initialPID = pid
		return true
	}

	// If we didn't find the browser process now but we previously saw it, then it probably crashed.
	if pid == -1 {
		bw.browserErr = fmt.Errorf("browser process %d exited", bw.initialPID)
		return false
	}

	// If the browser's PID changed, then it probably crashed and got restarted between checks.
	if pid != bw.initialPID {
		bw.browserErr = fmt.Errorf("browser process %d replaced by %d", bw.initialPID, pid)
		return false
	}

	// The original browser process is still running, so keep checking.
	return true
}
