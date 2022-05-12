// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacrosinfo allows inspection of the system's Lacros configuration
package lacrosinfo

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// Info represents the format returned from autotestPrivate.getLacrosInfo.
type Info struct {
	// True iff lacros is running.  Note that this information is a snapshot at a
	// particular time. That is, even if the info says lacros is running, it
	// doesn't necessarily mean lacros is still running at any particular time.
	Running bool `json:"isRunning"`
	// True iff lacros has keep-alive enabled..  Note that this information is a
	// snapshot at a particular time.
	KeepAlive bool `json:"isKeepAlive"`
	// Contains the path to the lacros directory - this is where lacros will be
	// executed from. Note that this may change over time if omaha is used (even
	// during a test). This also may be empty is lacros is not running.
	LacrosPath string `json:"lacrosPath"`
}

// Snapshot gets the current lacros info from ash-chrome. The parameter tconn should be the ash TestConn.
func Snapshot(ctx context.Context, tconn *chrome.TestConn) (*Info, error) {
	var info Info
	if err := tconn.Call(ctx, &info, "tast.promisify(chrome.autotestPrivate.getLacrosInfo)"); err != nil {
		return nil, err
	}
	return &info, nil
}
