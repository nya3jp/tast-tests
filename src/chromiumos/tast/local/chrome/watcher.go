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
	initialPID        int32      // first browser PID that was seen; initially -1
	sessionManagerPID int32      // the session manager PID that was seen; initially -1
	browserErr        error      // error that was detected, if any
	mutex             sync.Mutex // protects browserErr
	done              chan bool  // used to tell the watcher's goroutine to exit
	closed            chan error // used to wait for the goroutine to exit
}

// newBrowserWatcher creates a new browserWatcher and starts it.
func newBrowserWatcher() *browserWatcher {
	bw := &browserWatcher{initialPID: -1, sessionManagerPID: -1, done: make(chan bool, 1), closed: make(chan error, 1)}
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

// check is an internal method that checks the browser process, updating initialPID and browserErr as needed.
// Returns false after an error has been encountered, indicating that no further calls are needed.
func (bw *browserWatcher) check() bool {
	// Once the browser process ID is known, just check if the process ID is valid
	// (i.e. the browser process is still running).
	if bw.initialPID != -1 {
		// Creating an instance of process.Process to check if the PID is still
		// valid or not. Note that Process.IsRunning() can't be used since it is not
		// yet implemented for Linux.
		// TODO(mukai): update gopsutil package and replace this by IsRunning.
		if _, err := process.NewProcess(bw.initialPID); err != nil {
			bw.mutex.Lock()
			defer bw.mutex.Unlock()
			bw.browserErr = errors.Wrapf(err, "browser process %d exited; Chrome probably crashed", bw.initialPID)
			return false
		}
		// Next, check the existence of the session manager process by creating an
		// instance of process.Process.
		if _, err := process.NewProcess(bw.sessionManagerPID); err != nil {
			bw.mutex.Lock()
			defer bw.mutex.Unlock()
			bw.browserErr = errors.Wrapf(err, "session manager process %d exited; session manager probably crashed", bw.sessionManagerPID)
			return false
		}
		// Theoretically, there's a chance that both browser process and session
		// manager have finished and new processes come up with the same PID, though
		// it would be rare.
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
		bw.browserErr = errors.Wrapf(err, "browser process %d exited; Chrome probably crashed", pid)
		return false
	}
	ppid, err := proc.Ppid()
	if err != nil {
		bw.mutex.Lock()
		defer bw.mutex.Unlock()
		bw.browserErr = errors.Wrap(err, "failed to find the PID of the session manager")
	}
	bw.initialPID = int32(pid)
	bw.sessionManagerPID = ppid

	return true
}
