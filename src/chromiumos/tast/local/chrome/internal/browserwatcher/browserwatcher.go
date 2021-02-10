// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package browserwatcher provides a utility to monitor browser process for
// crashes.
package browserwatcher

import (
	"context"
	"sync"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/internal/chromeproc"
	"chromiumos/tast/testing"
)

const (
	checkBrowserInterval = 100 * time.Millisecond // interval to check browser process
)

// Watcher watches the browser process to attempt to identify situations where Chrome is crashing.
type Watcher struct {
	done   chan bool  // used to tell the watcher's goroutine to exit
	closed chan error // used to wait for the goroutine to exit

	initialPID        int32 // first browser PID that was seen; initially -1
	sessionManagerPID int32 // the session manager PID that was seen; initially -1

	mutex      sync.Mutex // protects browserErr
	browserErr error      // error that was detected, if any
}

// NewWatcher creates a new Watcher and starts it.
// Returns only when the chrome PID it's watching has been identified.
func NewWatcher(ctx context.Context) (*Watcher, error) {
	bw := &Watcher{
		done:              make(chan bool, 1),
		closed:            make(chan error, 1),
		initialPID:        -1,
		sessionManagerPID: -1,
	}
	go func() {
		defer func() {
			bw.closed <- bw.Err()
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

	// Ensure the chrome root PID has been identified before returning.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if bw.initialPID == -1 {
			return errors.New("chrome PID has not been identified")
		}
		return nil
	}, &testing.PollOptions{Interval: checkBrowserInterval}); err != nil {
		return nil, err
	}

	return bw, nil
}

// Close synchronously stops the watch goroutine.
func (bw *Watcher) Close() error {
	bw.done <- true
	return <-bw.closed
}

// Err returns the first error that was observed or nil if no error was observed.
func (bw *Watcher) Err() error {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()
	return bw.browserErr
}

// ReplaceErr returns the first error that was observed if any. Otherwise, it
// returns err as-is.
func (bw *Watcher) ReplaceErr(err error) error {
	if werr := bw.Err(); werr != nil {
		return werr
	}
	return err
}

// WaitExit polls until the *Watcher's initialPID is no longer running.
func (bw *Watcher) WaitExit(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := process.NewProcess(bw.initialPID); err == nil {
			return errors.New("chrome PID is still running")
		}
		return nil
	}, &testing.PollOptions{Interval: checkBrowserInterval})
}

// check is an internal method that checks the browser process, updating initialPID and browserErr as needed.
// Returns false after an error has been encountered, indicating that no further calls are needed.
func (bw *Watcher) check() bool {
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

	pid, err := chromeproc.GetRootPID()
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
