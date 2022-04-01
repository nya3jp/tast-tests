// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package playbilling

import (
	"context"
	"fmt"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/webapk"
)

// TestApp represents the Play Billing test PWA and ARC Payments Overlay.
type TestApp struct {
	cr          *chrome.Chrome
	pbconn      *chrome.Conn
	uiAutomator *ui.Device
	wm          *webapk.Manager
}

// NewTestApp returns a reference to a new Play Billing Test App.
func NewTestApp(ctx context.Context, arc *arc.ARC, d *ui.Device, wm *webapk.Manager) *TestApp {
	return &TestApp{
		pbconn:      nil,
		uiAutomator: d,
		wm:          wm,
	}
}

// Launch starts a new TestApp window.
func (ta *TestApp) Launch(ctx context.Context) error {
	if err := ta.wm.LaunchApp(ctx); err != nil {
		return errors.Wrap(err, "failed launching the test app")
	}

	pbconn, err := ta.wm.GetChromeConnection(ctx)
	if err != nil {
		return errors.Wrap(err, "failed getting connection for the test app")
	}

	ta.pbconn = pbconn

	return nil
}

// OpenBillingDialog clicks a button on the PWA to launch the Play Billing UI.
func (ta *TestApp) OpenBillingDialog(ctx context.Context, sku string) error {
	jsExpr := fmt.Sprintf("document.getElementById('%s')", sku)
	return ClickElementByCDP(ta.pbconn, jsExpr)(ctx)
}

// BuySku clicks the buy button on a Play Billing overlay.
func (ta *TestApp) BuySku(ctx context.Context) error {
	return ClickButtonOnArcPaymentOverlay(ta.uiAutomator, "Button", "Buy")(ctx)
}

// RequiredAuthConfirm clicks "Yes, always" button in required auth window.
func (ta *TestApp) RequiredAuthConfirm(ctx context.Context) error {
	return RequiredAuthConfirm(ta.uiAutomator)(ctx)
}

// CheckPaymentSuccessful checks for a presence of payment successful screen in the android ui tree.
func (ta *TestApp) CheckPaymentSuccessful(ctx context.Context) error {
	return CheckPaymentSuccessful(ta.uiAutomator)(ctx)
}
