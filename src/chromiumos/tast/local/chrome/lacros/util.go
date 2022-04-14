// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

var pollOptions = &testing.PollOptions{Timeout: 10 * time.Second}

// WaitForLacrosWindow waits for a Lacros window to be open and have the title to be visible if it is specified as a param.
func WaitForLacrosWindow(ctx context.Context, tconn *chrome.TestConn, title string) error {
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		if !w.IsVisible {
			return false
		}
		if !strings.HasPrefix(w.Name, "ExoShellSurface") {
			return false
		}
		if len(title) > 0 {
			return strings.HasPrefix(w.Title, title)
		}
		return true
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to wait for lacros-chrome window to be visible (title: %v)", title)
	}
	return nil
}

// CloseLacros closes the given lacros-chrome, if it is non-nil. Otherwise, it does nothing.
func CloseLacros(ctx context.Context, l *Lacros) {
	if l != nil {
		l.Close(ctx) // Ignore error.
	}
}

// PidsFromPath returns the pids of all processes with a given path in their
// command line. This is typically used to find all chrome-related binaries,
// e.g. chrome, nacl_helper, etc. They typically share a path, even though their
// binary names differ.
// There may be a race condition between calling this method and using the pids
// later. It's possible that one of the processes is killed, and possibly even
// replaced with a process with the same pid.
func PidsFromPath(ctx context.Context, path string) ([]int, error) {
	all, err := process.Pids()
	if err != nil {
		return nil, err
	}

	pids := make([]int, 0)
	for _, pid := range all {
		if proc, err := process.NewProcess(pid); err != nil {
			// Assume that the process exited.
			continue
		} else if exe, err := proc.Exe(); err == nil && strings.Contains(exe, path) {
			pids = append(pids, int(pid))
		}
	}
	return pids, nil
}

// Info represents the format returned from autotestPrivate.getLacrosInfo.
type Info struct {
	// True iff lacros is running.  Note that this information is a snapshot at a
	// particular time. That is, even if the info says lacros is running, it
	// doesn't necessarily mean lacros is still running at any particular time.
	Running bool `json:"isRunning"`
	// Contains the path to the lacros directory - this is where lacros will be
	// executed from. Note that this may change over time if omaha is used (even
	// during a test). This also may be empty is lacros is not running.
	LacrosPath string `json:"lacrosPath"`
}

// InfoSnapshot gets the current lacros info from ash-chrome. The parameter tconn should be the ash TestConn.
func InfoSnapshot(ctx context.Context, tconn *chrome.TestConn) (*Info, error) {
	var info Info
	if err := tconn.Call(ctx, &info, "tast.promisify(chrome.autotestPrivate.getLacrosInfo)"); err != nil {
		return nil, err
	}
	return &info, nil
}
