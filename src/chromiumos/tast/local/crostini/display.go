// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// The Size object records sizes of various display-related objects (e.g. the screen resolution, a window's size).
type Size struct {
	W int `json:"width"`
	H int `json:"height"`
}

// PollWindowSize returns the the width and the height of the window in pixels
// with polling to wait for asynchronous rendering on the DUT.
func PollWindowSize(ctx context.Context, tconn *chrome.Conn, name string) (sz Size, err error) {
	// Allow up to 10 seconds for the target screen to render.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		sz, err = getWindowSize(ctx, tconn, name)
		return err
	}, &testing.PollOptions{Timeout: 10 * time.Second})
	return sz, err
}

// getWindowSize returns the the width and the height of the window in pixels.
func getWindowSize(ctx context.Context, tconn *chrome.Conn, name string) (sz Size, err error) {
	expr := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.automation.getDesktop(root => {
				const appWindow = root.find({ attributes: { name: %q}});
				if (!appWindow) {
					reject("Failed to locate the app window");
				}
				const view = appWindow.find({ attributes: { className: 'ClientView'}});
				if (!view) {
					reject("Failed to find client view");
				}
				resolve(view.location);
			})
		})`, name)
	err = tconn.EvalPromise(ctx, expr, &sz)
	return sz, err
}

// PrimaryDisplayScaleFactor returns the primary display's scale factor.
func PrimaryDisplayScaleFactor(ctx context.Context, tconn *chrome.Conn) (factor float64, err error) {
	err = tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.getPrimaryDisplayScaleFactor)()`, &factor)
	return factor, err
}

// TabletModeEnabled returns whether tablet mode is enabled on the device.
func TabletModeEnabled(ctx context.Context, tconn *chrome.Conn) (tabletMode bool, err error) {
	err = tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.isTabletModeEnabled)()`, &tabletMode)
	return tabletMode, err
}
