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

	"github.com/shirou/gopsutil/v3/process"

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

	proc *process.Process // browser process.

	mutex      sync.Mutex // protects browserErr
	browserErr error      // error that was detected, if any
}

// NewWatcher creates a new Watcher and starts it.
// Returns only when the chrome PID it's watching has been identified.
func NewWatcher(ctx context.Context, execPath string) (*Watcher, error) {
	// Wait for the browser process to start.
	var proc *process.Process
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		proc, err = chromeproc.Root(execPath)
		if err != nil {
			return errors.Wrap(err, "browser process not started yet")
		}
		return nil
	}, &testing.PollOptions{Interval: checkBrowserInterval}); err != nil {
		return nil, err
	}

	bw := &Watcher{
		done:   make(chan bool, 1),
		closed: make(chan error, 1),
		proc:   proc,
	}

	go func() {
		defer func() {
			bw.closed <- bw.Err()
		}()
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

// WaitExit polls until the *Watcher's target process is no longer running.
func (bw *Watcher) WaitExit(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if running, err := bw.proc.IsRunning(); err == nil && running {
			return errors.New("chrome is still running")
		}
		return nil
	}, &testing.PollOptions{Interval: checkBrowserInterval})
}

// check is an internal method that checks the browser process, updating browserErr as needed.
// Returns false after an error has been encountered, indicating that no further calls are needed.
func (bw *Watcher) check() bool {
	if running, err := bw.proc.IsRunning(); err != nil || !running {
		bw.mutex.Lock()
		defer bw.mutex.Unlock()
		bw.browserErr = errors.Wrapf(err, "browser process %d exited; Chrome probably crashed", bw.proc.Pid)
		return false
	}
	return true
}
