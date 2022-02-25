// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// Level describes the power level management to use.
type Level int

const (
	// Display keeps the screen and system active.
	Display Level = iota
	// System keeps the system active but allows the screen to be dimmed or turned off.
	System
)

// RequestKeepAwake sends a request to keep the system awake. A function is
// returned to restore the previous state. Full reference can be found here:
// https://developer.chrome.com/docs/extensions/reference/power/.
func RequestKeepAwake(ctx context.Context, tconn *chrome.TestConn, level Level) (func(ctx context.Context, tconn *chrome.TestConn) error, error) {
	// Convert the level to a string for the API call.
	var levelVal string
	switch level {
	case Display:
		levelVal = "display"
	case System:
		levelVal = "system"
	default:
		return nil, errors.Errorf("invalid level provided: %v", level)
	}

	// Make a call to the requestKeepAwake API. Wrapping in a promisify
	// incorrectly sends the arguments which is why a custom handler is made.
	if err := tconn.Call(ctx, nil, `(level) => new Promise((resolve, reject) => {
		chrome.power.requestKeepAwake(level);
		if (chrome.runtime.lastError) {
			reject(new Error(chrome.runtime.lastError.message));
			return;
		}
		resolve();
	})`, levelVal); err != nil {
		return nil, errors.Wrap(err, "failed to call requestKeepAwake")
	}

	return func(ctx context.Context, tconn *chrome.TestConn) error {
		return releaseKeepAwake(ctx, tconn)
	}, nil
}

// releaseKeepAwake restores the previous power state. Full reference can be
// found here: https://developer.chrome.com/docs/extensions/reference/power/.
func releaseKeepAwake(ctx context.Context, tconn *chrome.TestConn) error {
	if err := tconn.Call(ctx, nil, `() => new Promise((resolve, reject) => {
			chrome.power.releaseKeepAwake();
			if (chrome.runtime.lastError) {
				reject(new Error(chrome.runtime.lastError.message));
				return;
			}
			resolve();
		})`); err != nil {
		return errors.Wrap(err, "failed to call releaseKeepAwake")
	}

	return nil
}
