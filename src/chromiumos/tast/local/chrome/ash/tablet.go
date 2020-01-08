// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"fmt"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
)

// SetTabletModeEnabled enables / disables tablet mode.
// After calling this function, it won't be possible to physically switch to/from tablet mode since that functionality will be disabled.
func SetTabletModeEnabled(ctx context.Context, c *chrome.Conn, enabled bool) error {
	e := strconv.FormatBool(enabled)
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setTabletModeEnabled(%s, function(enabled) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
		    if (enabled != %s) {
		      reject(new Error("unexpected tablet mode: " + enabled));
		    } else {
		      resolve();
		    }
		  })
		})`, e, e)
	return c.EvalPromise(ctx, expr, nil)
}

// TabletModeEnabled gets the tablet mode enabled status.
func TabletModeEnabled(ctx context.Context, tconn *chrome.Conn) (bool, error) {
	var enabled bool
	err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.isTabletModeEnabled)()`, &enabled)
	return enabled, err
}

// EnsureTabletModeEnabled makes sure that the tablet mode state is |enabled|,
// and returns a function which reverts back to the original state.
// Typically, this will be used like:
//   cleanup, err := ash.EnsureTabletModeEnabled(ctx, c, true)
//   if err != nil {
//     s.Fatal("Failed to ensure in tablet mode: ", err)
//   }
//   defer cleanup(ctx)
func EnsureTabletModeEnabled(ctx context.Context, c *chrome.Conn, enabled bool) (func(ctx context.Context) error, error) {
	originallyEnabled, err := TabletModeEnabled(ctx, c)
	if err != nil {
		return nil, err
	}
	var mouse *input.MouseEventWriter
	if originallyEnabled != enabled {
		// Creates a mouse device if it is ensuring clamshell mode on a tablet
		// device. Without a mouse device, Ash could return to the tablet mode
		// automatically on certain condition (like display rotation).
		if !enabled {
			if mouse, err = input.Mouse(ctx); err != nil {
				return nil, errors.Wrap(err, "failed to set up mouse")
			}
		}
		if err = SetTabletModeEnabled(ctx, c, enabled); err != nil {
			return nil, err
		}
	}
	// Always revert to the original state; so it can always be back to the original
	// state even when the state changes in another part of the test script.
	return func(ctx context.Context) error {
		if mouse != nil {
			mouse.Close()
		}
		return SetTabletModeEnabled(ctx, c, originallyEnabled)
	}, nil
}
