// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package browserwatcher provides a utility to monitor browser process for
// crashes.
package browserwatcher

import (
	"context"
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
	proc       *process.Process // browser process
	browserErr error            // error that was detected, if any
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

	return &Watcher{proc: proc}, nil
}

// ReplaceErr returns the first error that was observed if any. Otherwise, it
// returns err as-is.
func (bw *Watcher) ReplaceErr(inErr error) error {
	if bw.browserErr == nil {
		// Check both running and err here to avoid edge cases
		// like recycling process ID.
		// See IsRunning implementation for details.
		running, err := bw.proc.IsRunning()
		if err == nil && running {
			// No error is found, so return the original error.
			return inErr
		}
		bw.browserErr = errors.Wrapf(err, "browser process %d exited; Chrome probably crashed", bw.proc.Pid)
	}
	// Some error was found, so return it instead of the given err.
	return bw.browserErr
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
