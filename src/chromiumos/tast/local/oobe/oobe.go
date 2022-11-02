// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package oobe contains helpers for shared logic in OOBE related tests.
package oobe

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// IsWelcomeScreenVisible checks the current page in OOBE to see if it's
// currently on the welcome page.
func IsWelcomeScreenVisible(ctx context.Context, oobeConn *chrome.Conn) error {
	return oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()")
}

// ClickHidScreenNextButton clicks on the next button in OOBE HID detection screen.
func ClickHidScreenNextButton(ctx context.Context, ui *uiauto.Context, oobeConn *chrome.Conn) error {

	var nextbuttonName string
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.getNextButtonName()", &nextbuttonName); err != nil {
		return errors.Wrap(err, "failed to retrieve the next button")
	}

	if err := uiauto.Combine("Click on next button",
		ui.WaitUntilExists(nodewith.Name(nextbuttonName).Role(role.Button)),
		ui.LeftClick(nodewith.Name(nextbuttonName).Role(role.Button)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on next button")
	}

	return nil
}
