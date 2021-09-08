// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
)

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
