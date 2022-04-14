// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package playbilling

import (
	"context"
	"fmt"
	"time"

	"github.com/mafredri/cdp/protocol/input"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
)

const (
	uiTimeout = 30 * time.Second
)

type elementCoordinates struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// waitForStableElementByJs waits until location of the element defined by jsExpr is stable.
func waitForStableElementByJs(ctx context.Context, conn *chrome.Conn, jsExpr string) (elementCoordinates, error) {
	var previousLocation, currentLocation elementCoordinates
	start := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.Eval(ctx, fmt.Sprintf(`new Promise(resolve => {
			const domRect = %s.getBoundingClientRect();
			resolve({
				x: domRect.x,
				y: domRect.y,
			});
		})`, jsExpr), &currentLocation); err != nil {
			previousLocation = elementCoordinates{}
			return err
		}
		if currentLocation != previousLocation {
			previousLocation = currentLocation
			elapsed := time.Since(start)
			return errors.Errorf("element has not stopped changing location after %s", elapsed)
		}
		return nil
	}, &testing.PollOptions{Timeout: uiTimeout, Interval: time.Second}); err != nil {
		return elementCoordinates{}, errors.Wrapf(err, "failed to wait for %q to stabilize", jsExpr)
	}

	return currentLocation, nil
}

// checkPresenceOfArcObject checks for the presence of an object in the android ui tree.
func checkPresenceOfArcObject(uiAutomator *ui.Device, objectType, objectText string) action.Action {
	return func(ctx context.Context) error {
		object := uiAutomator.Object(ui.ClassName(objectType), ui.TextMatches(objectText), ui.Enabled(true))
		return object.WaitForExists(ctx, uiTimeout)
	}
}

// ClickElementByCDP clicks the element defined by jsExpr.
// A separate function to emulate a click is needed, because there are restrictions,
// which don't allow invoking billing actions via js interactions.
func ClickElementByCDP(conn *chrome.Conn, jsExpr string) action.Action {
	return func(ctx context.Context) error {
		if err := conn.WaitForExprFailOnErrWithTimeout(ctx, fmt.Sprintf("%s != undefined", jsExpr), uiTimeout); err != nil {
			return errors.Wrapf(err, "failed to wait for %q to be defined", jsExpr)
		}

		// Scroll the element into the view.
		// Otherwise CDP event will fail to click it.
		if err := conn.Eval(ctx, fmt.Sprintf("%s.scrollIntoViewIfNeeded()", jsExpr), nil); err != nil {
			return errors.Wrapf(err, "failed to scroll %v into view", jsExpr)
		}

		currentLocation, err := waitForStableElementByJs(ctx, conn, jsExpr)
		if err != nil {
			return err
		}

		// Dispatch a CDP mouse event to click and release relative to the PWA window.
		// Can't call the JS API as require gesture to initiate mouse click.
		mousePressed := input.NewDispatchMouseEventArgs("mousePressed", currentLocation.X, currentLocation.Y).SetClickCount(1).SetButton(input.MouseButtonLeft)
		if err := conn.DispatchMouseEvent(ctx, mousePressed); err != nil {
			return errors.Wrapf(err, "failed to mouse down at %v", currentLocation)
		}
		mousePressed.Type = "mouseReleased"
		if err := conn.DispatchMouseEvent(ctx, mousePressed); err != nil {
			return errors.Wrapf(err, "failed to mouse up at %v", currentLocation)
		}

		return nil
	}
}

// ClickButtonOnArcPaymentOverlay clicks a button and text.
func ClickButtonOnArcPaymentOverlay(uiAutomator *ui.Device, buttonType, objectText string) action.Action {
	return func(ctx context.Context) error {
		button := uiAutomator.Object(ui.ClassName("android.widget."+buttonType), ui.TextMatches(objectText), ui.Enabled(true))
		if err := button.WaitForExists(ctx, uiTimeout); err != nil {
			return errors.Wrapf(err, "failed waiting %v for button to exist", uiTimeout)
		}
		return button.Click(ctx)
	}
}

// Click1TapBuy clicks 1-tap buy button.
func Click1TapBuy(uiAutomator *ui.Device) action.Action {
	return ClickButtonOnArcPaymentOverlay(uiAutomator, "Button", "1-tap buy")
}

// RequiredAuthConfirm clicks "Yes, always" button in required auth window.
func RequiredAuthConfirm(uiAutomator *ui.Device) action.Action {
	return action.IfSuccessThen(
		checkPresenceOfArcObject(uiAutomator, "android.widget.TextView", `Require authentication for purchases\?`),
		uiauto.Combine("confirm required authentication",
			ClickButtonOnArcPaymentOverlay(uiAutomator, "RadioButton", "Yes, always"),
			ClickButtonOnArcPaymentOverlay(uiAutomator, "Button", "OK"),
		),
	)
}

// TapPointsDecline declines tap points proposal, if available.
func TapPointsDecline(uiAutomator *ui.Device) action.Action {
	return action.IfSuccessThen(
		checkPresenceOfArcObject(uiAutomator, "android.widget.TextView", "See Google Play Points terms"),
		ClickButtonOnArcPaymentOverlay(uiAutomator, "Button", "Not now"),
	)
}

// AlreadyOwnErrorClose closes "You already own this item" window.
func AlreadyOwnErrorClose(uiAutomator *ui.Device) action.Action {
	return uiauto.Combine("close error window",
		checkPresenceOfArcObject(uiAutomator, "android.widget.TextView", `You already own this item\.`),
		ClickButtonOnArcPaymentOverlay(uiAutomator, "Button", "OK"),
	)
}

// CheckPaymentSuccessful checks for a presence of payment successful screen in the android ui tree.
func CheckPaymentSuccessful(uiAutomator *ui.Device) action.Action {
	return checkPresenceOfArcObject(uiAutomator, "android.widget.TextView", "Payment successful")
}
