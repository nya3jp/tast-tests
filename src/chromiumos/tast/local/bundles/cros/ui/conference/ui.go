// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// doFullScreenAction returns an action that does the given fullScreenAction.
func doFullScreenAction(tconn *chrome.TestConn, fullScreenAction action.Action, title string, isFullScreen bool) action.Action {
	ui := uiauto.New(tconn)
	actionDescription := "enter full screen"
	if !isFullScreen {
		actionDescription = "exit full screen"
	}
	return uiauto.NamedAction(actionDescription,
		ui.Retry(3, uiauto.Combine(actionDescription,
			fullScreenAction,
			waitForFullscreenCondition(tconn, title, isFullScreen),
		)))
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

// allowPagePermissions checks whether the page has been blocked.
// If the page has been blocked, unblock camera and microphone permissions.
func allowPagePermissions(tconn *chrome.TestConn) action.Action {
	ui := uiauto.New(tconn)
	blockedButton := nodewith.NameContaining("This page has been blocked").Role(role.Button)
	dialogRoot := nodewith.Name("Camera and microphone blocked").Role(role.Window).ClassName("ContentSettingBubbleContents")
	alwaysAllowButton := nodewith.NameContaining("Always allow").Role(role.RadioButton).Ancestor(dialogRoot)
	doneButton := nodewith.Name("Done").Role(role.Button).Focusable().Ancestor(dialogRoot)
	reloadButton := nodewith.Name("Reload").Role(role.Button).Focusable().First()
	accessButton := nodewith.NameContaining("This page is accessing").Role(role.Button)
	allowPermission := uiauto.NamedAction("allow page permissions",
		uiauto.Combine("allow page permissions",
			ui.LeftClick(blockedButton),
			ui.LeftClick(alwaysAllowButton),
			ui.LeftClick(doneButton),
			ui.LeftClick(reloadButton),
			ui.WaitUntilExists(accessButton),
		))
	return uiauto.IfSuccessThen(ui.WithTimeout(3*time.Second).WaitUntilExists(blockedButton), allowPermission)
}

// takeScreenshot returns an action which captures a fullscreen screenshot.
func takeScreenshot(cr *chrome.Chrome, outDir, name string) action.Action {
	return func(ctx context.Context) error {
		path := fmt.Sprintf("%s/screenshot-%s.png", outDir, name)
		if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
			testing.ContextLog(ctx, "Failed to capture screenshot: ", err)
		}
		return nil
	}
}
