// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package oobe contains helpers for shared logic in OOBE related tests.
package oobe

import (
	"context"
	"time"

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

// IsHidDetectionKeyboardSearchingForKeyboard checks if OOBE HID Detection page is searching for keyboard device.
func IsHidDetectionKeyboardSearchingForKeyboard(ctx context.Context, oobeConn *chrome.Conn, tconn *chrome.TestConn) error {
	var keyboardNotDetectedText string
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.getKeyboardNotDetectedText()", &keyboardNotDetectedText); err != nil {
		return err
	}
	keyboardNotDetectedTextNode := nodewith.Role(role.StaticText).Name(keyboardNotDetectedText)
	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)
	return ui.WaitUntilExists(keyboardNotDetectedTextNode)(ctx)
}

// IsHidDetectionSearchingForMouse checks if OOBE HID Detection page is searching for mouse device.
func IsHidDetectionSearchingForMouse(ctx context.Context, oobeConn *chrome.Conn, tconn *chrome.TestConn) error {
	var mouseNotDetectedText string
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.getMouseNotDetectedText()", &mouseNotDetectedText); err != nil {
		return err
	}
	mouseNotDetectedTextNode := nodewith.Role(role.StaticText).Name(mouseNotDetectedText)
	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)
	return ui.WaitUntilExists(mouseNotDetectedTextNode)(ctx)
}

// ClickHidScreenNextButton clicks on the next button in OOBE HID detection screen.
func ClickHidScreenNextButton(ctx context.Context, oobeConn *chrome.Conn, tconn *chrome.TestConn) error {

	var nextbuttonName string
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.getNextButtonName()", &nextbuttonName); err != nil {
		return errors.Wrap(err, "failed to retrieve the next button")
	}

	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)
	if err := uiauto.Combine("Click on next button",
		ui.WaitUntilExists(nodewith.Name(nextbuttonName).Role(role.Button)),
		ui.LeftClick(nodewith.Name(nextbuttonName).Role(role.Button)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on next button")
	}

	return nil
}
