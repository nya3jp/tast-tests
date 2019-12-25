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

// StartWindowCycling starts alt-tab window cycling.
// If showCycler is true, the UI is shown and you can select the window to activate by calling |CycleWindows|.
// If showCycler is false, the next activatable window is activated immediately.
// It's the responsibility of the caller of this function to complete the process by calling |CompleteWindowCycling|.
func StartWindowCycling(ctx context.Context, c *chrome.Conn, showCycler bool) error {
	e := strconv.FormatBool(showCycler)
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.startWindowCycling(%s, function() {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
			resolve();
		  })
		})`, e)
	return c.EvalPromise(ctx, expr, nil)
}

// CycleWindows mvoes the selection of the window to activate to the next in the cycler.
func CycleWindows(ctx context.Context, c *chrome.Conn) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.cycleWindows(function() {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
			resolve();
		  })
		})`)
	return c.EvalPromise(ctx, expr, nil)
}

// CompleteWindowCycling complete the current window cycling and activate one of the windows.
func CompleteWindowCycling(ctx context.Context, c *chrome.Conn) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.completeWindowCycling(function() {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
			resolve();
		  })
		})`)
	return c.EvalPromise(ctx, expr, nil)
}
