// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacrosinfo allows querying Ash about the state of Lacros
package lacrosinfo

import (
	"context"

	"chromiumos/tast/local/chrome"
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
func Snapshot(ctx context.Context, tconn *chrome.TestConn) (*Info, error) {
	var info Info
	if err := tconn.Call(ctx, &info, "tast.promisify(chrome.autotestPrivate.getLacrosInfo)"); err != nil {
		return nil, err
	}
	return &info, nil
}
