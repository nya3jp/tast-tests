// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros is an internal package (to chrome) that exists solely to
// define the ResetState function (resetting Lacros state) such that it can be
// used in chrome.ResetState without causing an import cycle.
// Its contents are reexported as part of the public packages lacros and
// lacrosinfo.
package lacros

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/internal/chromeproc"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/testing"
)

const (
	// UserDataDir is the directory that contains the user data of lacros.
	UserDataDir = "/home/chronos/user/lacros/"
)

// LacrosState represents the state of Lacros. See BrowserManager::State in Chromium's browser_manager.h.
type LacrosState string

// LacrosState values. To be extended on demand.
const (
	LacrosStateStopped LacrosState = "Stopped"
)

// LacrosMode represents the mode of Lacros. See crosapi::browser_util::LacrosMode.
type LacrosMode string

// LacrosMode values.
const (
	LacrosModeDisabled   LacrosMode = "Disabled"
	LacrosModeSideBySide LacrosMode = "SideBySide"
	LacrosModePrimary    LacrosMode = "Primary"
	LacrosModeOnly       LacrosMode = "Only"
)

// Info represents the format returned from autotestPrivate.getLacrosInfo.
type Info struct {
	// The state Lacros is in.  Note that this information is a snapshot at a
	// particular time and may thus be outdated already.
	State LacrosState `json:"state"`
	// True iff lacros has keep-alive enabled.  Note that this information is a
	// snapshot at a particular time.
	KeepAlive bool `json:"isKeepAlive"`
	// Contains the path to the lacros directory - this is where lacros will be
	// executed from. Note that this may change over time if omaha is used (even
	// during a test). This also may be empty is lacros is not running.
	LacrosPath string `json:"lacrosPath"`
	// The mode of Lacros.
	Mode LacrosMode `json:"mode"`
}

// Snapshot gets the current lacros info from ash-chrome. The parameter tconn should be the ash TestConn.
func Snapshot(ctx context.Context, tconn *driver.TestConn) (*Info, error) {
	var info Info
	if err := tconn.Call(ctx, &info, "tast.promisify(chrome.autotestPrivate.getLacrosInfo)"); err != nil {
		return nil, err
	}
	return &info, nil
}

// ResetState terminates Lacros and removes its user data directory, unless KeepAlive is enabled.
func ResetState(ctx context.Context, tconn *driver.TestConn) error {
	info, err := Snapshot(ctx, tconn)
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
