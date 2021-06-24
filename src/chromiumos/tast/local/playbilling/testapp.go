// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package playbilling

import (
	"context"
	"fmt"
	"time"

	"github.com/mafredri/cdp/protocol/input"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// TestApp represents the Play Billing test PWA and ARC Payments Overlay.
type TestApp struct {
	cr          *chrome.Chrome
	pbconn      *chrome.Conn
	tconn       *chrome.TestConn
	uiAutomator *ui.Device
}

const (
	appID              = "dlbmfdiobcnhnfocmenonncepnmhpckd"
	localServerAddress = "http://127.0.0.1/"
)

type elementCoordinates struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NewTestApp returns a reference to a new Play Billing Test App.
func NewTestApp(ctx context.Context, cr *chrome.Chrome, arc *arc.ARC, uiAutomator *ui.Device) (*TestApp, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed getting Test API connection")
	}

	return &TestApp{
		cr:          cr,
		tconn:       tconn,
		uiAutomator: uiAutomator,
		pbconn:      nil,
	}, nil
}

// Launch starts a new TestApp window.
func (ta *TestApp) Launch(ctx context.Context) error {
	if err := apps.Launch(ctx, ta.tconn, appID); err != nil {
		return errors.Wrapf(err, "failed launching app ID %q", appID)
	}

	pbconn, err := ta.cr.NewConnForTarget(ctx, chrome.MatchTargetURL(localServerAddress))
	if err != nil {
		return errors.Wrapf(err, "failed getting connection for target: %q", localServerAddress)
	}

	ta.pbconn = pbconn

	return nil
}

// OpenBillingDialog clicks a button on the PWA to launch the Play Billing UI.
func (ta *TestApp) OpenBillingDialog(ctx context.Context, sku string) error {
	return ta.clickElementByID(ctx, sku)
}

func (ta *TestApp) clickButtonOnArcPaymentOverlay(ctx context.Context, uiAutomator *ui.Device, buttonType, objectText string) error {
	button := uiAutomator.Object(ui.ClassName("android.widget."+buttonType), ui.TextMatches(objectText), ui.Enabled(true))
	if err := button.WaitForExists(ctx, 10*time.Second); err != nil {
		return errors.Wrapf(err, "failed waiting %v for button to exist", 10*time.Second)
	}
	return button.Click(ctx)
}

// BuySku clicks the buy button on a Play Billing overlay.
func (ta *TestApp) BuySku(ctx context.Context) error {
	return ta.clickButtonOnArcPaymentOverlay(ctx, ta.uiAutomator, "Button", "Buy")
}

func (ta *TestApp) waitForStableElementByID(ctx context.Context, id string) (elementCoordinates, error) {
	jsExpr := fmt.Sprintf("document.getElementById('%s')", id)
	if err := ta.pbconn.WaitForExprFailOnErrWithTimeout(ctx, fmt.Sprintf("%s != undefined", jsExpr), 30*time.Second); err != nil {
		return elementCoordinates{}, errors.Wrapf(err, "failed to wait for %q to be defined", jsExpr)
	}

	var previousLocation, currentLocation elementCoordinates
	start := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := ta.pbconn.Eval(ctx, fmt.Sprintf(`new Promise(resolve => {
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
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
		return elementCoordinates{}, errors.Wrapf(err, "failed to wait for %q to stabilize", jsExpr)
	}

	return currentLocation, nil
}

func (ta *TestApp) clickElementByID(ctx context.Context, id string) error {
	currentLocation, err := ta.waitForStableElementByID(ctx, id)
	if err != nil {
		return err
	}

	// Dispatch a CDP mouse event to click and release relative to the PWA window.
	// Can't call the JS API as require gesture to initiate mouse click.
	mousePressed := input.NewDispatchMouseEventArgs("mousePressed", currentLocation.X, currentLocation.Y).SetClickCount(1).SetButton(input.MouseButtonLeft)
	if err := ta.pbconn.DispatchMouseEvent(ctx, mousePressed); err != nil {
		return errors.Wrapf(err, "failed to mouse down at %v", currentLocation)
	}
	mousePressed.Type = "mouseReleased"
	if err := ta.pbconn.DispatchMouseEvent(ctx, mousePressed); err != nil {
		return errors.Wrapf(err, "failed to mouse up at %v", currentLocation)
	}

	return nil
}
