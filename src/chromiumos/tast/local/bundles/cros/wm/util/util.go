// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package util contains utility functions for window management.
package util

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// TakeFullScreenshot takes full screenshot by "Screen captrue" in the quick settings.
func TakeFullScreenshot(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	toggleBtn := nodewith.Role(role.ToggleButton)
	verifyText := nodewith.Name("Click anywhere to capture full screen")
	window := nodewith.Role(role.Window).First()

	return uiauto.Combine("take full screenshot",
		ui.LeftClick(toggleBtn.Name("Screenshot")),
		ui.LeftClick(toggleBtn.Name("Take full screen screenshot")),
		ui.WaitUntilExists(verifyText),
		ui.LeftClick(window), // Click on the center of root window to take the screenshot.
	)(ctx)
}
