// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"fmt"
	"strconv"

	"chromiumos/tast/local/chrome"
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

// ScopedTabletMode is a utility type to ensure the system mode into tablet mode
// (or clamshell mode). It can revert back to the original mode on Close().
// Typically, this will be used like:
//   tm, err := ash.EnsureTabletModeEnabled(ctx, c, true)
//   if err != nil {
//     s.Fatal("Failed to ensure in tablet mode: ", err)
//   }
//   defer tm.Close()
type ScopedTabletMode struct {
	originallyEnabled bool
	c                 *chrome.Conn
}

// EnsureTabletModeEnabled makes sure that the tablet mode state is |enabled|,
// and returns an instance of ScopedTabletMode.
func EnsureTabletModeEnabled(ctx context.Context, c *chrome.Conn, enabled bool) (*ScopedTabletMode, error) {
	originallyEnabled, err := TabletModeEnabled(ctx, c)
	if err != nil {
		return nil, err
	}
	if originallyEnabled != enabled {
		if err = SetTabletModeEnabled(ctx, c, enabled); err != nil {
			return nil, err
		}
	}
	return &ScopedTabletMode{originallyEnabled: originallyEnabled, c: c}, nil
}

// Close closes the current scope of tablet mode ensurance; it might request to
// the original state.
func (tm *ScopedTabletMode) Close(ctx context.Context) error {
	return SetTabletModeEnabled(ctx, tm.c, tm.originallyEnabled)
}
