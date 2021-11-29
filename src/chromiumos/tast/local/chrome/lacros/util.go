// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

var pollOptions = &testing.PollOptions{Timeout: 10 * time.Second}

func waitForWindowWithPredicate(ctx context.Context, ctconn *chrome.TestConn, p func(*ash.Window) bool) (*ash.Window, error) {
	if err := ash.WaitForCondition(ctx, ctconn, p, pollOptions); err != nil {
		return nil, err
	}
	return ash.FindWindow(ctx, ctconn, p)
}

// FindFirstBlankWindow finds the first window whose title is 'about:blank'.
func FindFirstBlankWindow(ctx context.Context, ctconn *chrome.TestConn) (*ash.Window, error) {
	return waitForWindowWithPredicate(ctx, ctconn, func(w *ash.Window) bool {
		return strings.Contains(w.Title, "about:blank")
	})
}

// FindFirstNonBlankWindow finds the first window whose title is not 'about:blank'.
func FindFirstNonBlankWindow(ctx context.Context, ctconn *chrome.TestConn) (*ash.Window, error) {
	return waitForWindowWithPredicate(ctx, ctconn, func(w *ash.Window) bool {
		return !strings.Contains(w.Title, "about:blank")
	})
}

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

// CloseLacrosChrome closes the given lacros-chrome, if it is non-nil. Otherwise, it does nothing.
func CloseLacrosChrome(ctx context.Context, l *LacrosChrome) {
	if l != nil {
		l.Close(ctx) // Ignore error.
	}
}

// killLacrosChrome kills all binaries whose executable contains the base path
// to lacros-chrome.
func killLacrosChrome(ctx context.Context, lacrosPath string) error {
	if lacrosPath == "" {
		return errors.New("Path to lacros-chrome cannot be empty")
	}

	// Kills all instances of lacros-chrome and other related executables.
	pids, err := PidsFromPath(ctx, lacrosPath)
	if err != nil {
		return errors.Wrap(err, "error finding pids for lacros-chrome")
	}
	for _, pid := range pids {
		// We ignore errors, since it's possible the process has
		// already been killed.
		unix.Kill(pid, syscall.SIGKILL)
	}
	return nil
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
