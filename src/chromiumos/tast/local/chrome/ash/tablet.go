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

// IsTabletModeEnabled gets the tablet mode enabled status.
func IsTabletModeEnabled(ctx context.Context, tconn *chrome.Conn) (bool, error) {
	var enabled bool
	err := tconn.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.isTabletModeEnabled(function(enabled) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(enabled);
		    }
		  })
		})`, &enabled)
	return enabled, err
}
