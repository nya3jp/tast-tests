// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package oobe contains helpers for shared logic in OOBE related tests.
package oobe

import (
	"context"
	"time"

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

// IsHidDetectionScreenVisible checks if the current page in OOBE to see if it's
// currently on the HID Detection page.
func IsHidDetectionScreenVisible(ctx context.Context, oobeConn *chrome.Conn) error {
	return oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.isVisible()")
}

// IsHidDetectionTouchscreenDetected checks if a touchscreen is detected in the
// OOBE HID Detection page.
func IsHidDetectionTouchscreenDetected(ctx context.Context, oobeConn *chrome.Conn) error {
	return oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.touchscreenDetected()")
}

// IsHidDetectionContinueButtonEnabled checks if the continue button is enabled
// in the OOBE HID Detection page.
func IsHidDetectionContinueButtonEnabled(ctx context.Context, oobeConn *chrome.Conn) error {
	return oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.canClickNext()")
}

// IsHidDetectionKeyboardNotDetected checks if there is no keyboard detected in
// the OOBE HID Detection page.
func IsHidDetectionKeyboardNotDetected(ctx context.Context, oobeConn *chrome.Conn, tconn *chrome.TestConn) error {
	var keyboardNotDetectedText string
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.getKeyboardNotDetectedText()", &keyboardNotDetectedText); err != nil {
		return err
	}
	keyboardNotDetectedTextNode := nodewith.Role(role.StaticText).Name(keyboardNotDetectedText)
	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)
	return ui.WaitUntilExists(keyboardNotDetectedTextNode)(ctx)
}

// IsHidMouseDetected checks if the current page in OOBE to see if it's
// currently on the HID Detection page.
func IsHidMouseDetected(ctx context.Context, oobeConn *chrome.Conn) error {
	return oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.mouseDetected()")
}

// IsHidKeyboardDetected checks if the current page in OOBE to see if it's
// currently on the HID Detection page.
func IsHidKeyboardDetected(ctx context.Context, oobeConn *chrome.Conn) error {
	return oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.keyboardDetected()")
}
