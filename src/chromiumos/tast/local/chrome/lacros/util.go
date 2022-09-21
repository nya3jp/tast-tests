// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"os"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/internal/chromeproc"
	"chromiumos/tast/local/chrome/lacros/lacrosinfo"
	"chromiumos/tast/local/procutil"
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

// ResetState terminates Lacros and removes its user data directory, unless KeepAlive is enabled.
func ResetState(ctx context.Context, tconn *chrome.TestConn) error {
	info, err := lacrosinfo.Snapshot(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve lacrosinfo")
	}
	if info.KeepAlive {
		testing.ContextLog(ctx, "Skipping resetting Lacros's state due to KeepAlive")
		return nil
	}

	testing.ContextLog(ctx, "Resetting Lacros's state")

	if len(info.LacrosPath) != 0 {
		lacrosProc, err := chromeproc.Root(info.LacrosPath + "/chrome")
		if err == procutil.ErrNotFound {
			// Lacros just terminated.
		} else if err != nil {
			return errors.Wrap(err, "failed to get Lacros process")
		} else {
			testing.ContextLog(ctx, "Lacros is still running, trying to terminate it now")
			lacrosProc.Terminate()
			if err := procutil.WaitForTerminated(ctx, lacrosProc, 3*time.Second); err != nil {
				return errors.Wrap(err, "failed to wait for process termination")
			}
		}
	}

	if err := os.RemoveAll(UserDataDir); err != nil {
		return errors.Wrap(err, "failed to delete Lacros user data directory")
	}

	return nil
}
