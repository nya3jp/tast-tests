// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/action"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

// expandMenu returns a function that clicks the button and waits for the menu to expand to the given height.
// This function is useful when the target menu will expand to its full size with animation. On Low end DUTs
// the expansion animation might stuck for some time. The node might have returned a stable location if
// checking with a fixed interval before the animiation completes. This function ensures animation completes
// by checking the menu height.
func expandMenu(tconn *chrome.TestConn, button, menu *nodewith.Finder, height int) action.Action {
	ui := uiauto.New(tconn)
	startTime := time.Now()
	return func(ctx context.Context) error {
		if err := ui.LeftClick(button)(ctx); err != nil {
			return errors.Wrap(err, "failed to click button")
		}
		return testing.Poll(ctx, func(ctx context.Context) error {
			menuInfo, err := ui.Info(ctx, menu)
			if err != nil {
				return errors.Wrap(err, "failed to get menu info")
			}
			if menuInfo.Location.Height < height {
				return errors.Errorf("got menu height %d, want %d", menuInfo.Location.Height, height)
			}
			// Examine this log regularly to see how fast the menu is expanded and determine if
			// we still need to keep this expandMenu() function.
			testing.ContextLog(ctx, "Menu expanded to full height in ", time.Now().Sub(startTime))
			return nil
		}, &testing.PollOptions{Timeout: 15 * time.Second, Interval: time.Second})
	}
}

// doFullScreenAction returns an action that does the given fullScreenAction.
func doFullScreenAction(tconn *chrome.TestConn, fullScreenAction action.Action, title string, isFullScreen bool) action.Action {
	ui := uiauto.New(tconn)
	return func(ctx context.Context) error {
		actionDescription := "enter full screen"
		if isFullScreen {
			testing.ContextLog(ctx, "Start to enter full screen")
		} else {
			testing.ContextLog(ctx, "Start to exit full screen")
			actionDescription = "exit full screen"
		}
		return ui.Retry(3, uiauto.Combine(actionDescription,
			fullScreenAction,
			waitForFullscreenCondition(tconn, title, isFullScreen),
		))(ctx)
	}
}

// waitForFullscreenCondition returns an action that waits for the expected window to
// be in full screen state if isFullScreen flag is true, or not in full screen state if
// the flag is false.
func waitForFullscreenCondition(tconn *chrome.TestConn, title string, isFullScreen bool) action.Action {
	return func(ctx context.Context) error {
		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			if !strings.Contains(w.Title, title) {
				return false
			}
			if isFullScreen {
				// Check the chrome window is in full screen.
				return w.State == ash.WindowStateFullscreen && !w.IsAnimating
			}
			// Check the chrome window is not in full screen.
			return w.State != ash.WindowStateFullscreen && !w.IsAnimating
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait for expected window state")
		}
		return nil
	}
}
