// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// SetTabletModeEnabled enables / disables tablet mode.
// After calling this function, it won't be possible to physically switch to/from tablet mode since that functionality will be disabled.
func SetTabletModeEnabled(ctx context.Context, tconn *chrome.TestConn, enabled bool) error {
	return tconn.Call(ctx, nil, `async (e) => {
	  const enabled = await tast.promisify(chrome.autotestPrivate.setTabletModeEnabled)(e);
	  if (enabled !== e)
	    throw new Error("unexpected tablet mode: " + enabled);
	}`, enabled)
}

// TabletModeEnabled gets the tablet mode enabled status.
func TabletModeEnabled(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	var enabled bool
	err := tconn.Call(ctx, &enabled, `tast.promisify(chrome.autotestPrivate.isTabletModeEnabled)`)
	return enabled, err
}

// EnsureTabletModeEnabled makes sure that the tablet mode state is enabled,
// and returns a function which reverts back to the original state.
//
// Typically, this will be used like:
//   cleanup, err := ash.EnsureTabletModeEnabled(ctx, c, true)
//   if err != nil {
//     s.Fatal("Failed to ensure in tablet mode: ", err)
//   }
//   defer cleanup(ctx)
func EnsureTabletModeEnabled(ctx context.Context, tconn *chrome.TestConn, enabled bool) (func(ctx context.Context) error, error) {
	originallyEnabled, err := TabletModeEnabled(ctx, tconn)
	if err != nil {
		return nil, err
	}
	if originallyEnabled != enabled {
		if err = SetTabletModeEnabled(ctx, tconn, enabled); err != nil {
			return nil, err
		}
	}
	// Always revert to the original state; so it can always be back to the original
	// state even when the state changes in another part of the test script.
	return func(ctx context.Context) error {
		return SetTabletModeEnabled(ctx, tconn, originallyEnabled)
	}, nil
}
