// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package playbilling

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
)

// TestApp represents the Play Billing test PWA and ARC Payments Overlay.
type TestApp struct {
	cr          *chrome.Chrome
	pbconn      *chrome.Conn
	tconn       *chrome.TestConn
	uiAutomator *ui.Device
}

const (
	appID              = "obcppbejhdfcplncjdlmagmpfjhmipii"
	localServerAddress = "http://127.0.0.1:8080/"
	uiTimeout          = 30 * time.Second
)

// NewTestApp returns a reference to a new Play Billing Test App.
func NewTestApp(ctx context.Context, cr *chrome.Chrome, arc *arc.ARC, d *ui.Device) (*TestApp, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed getting Test API connection")
	}

	return &TestApp{
		cr:          cr,
		tconn:       tconn,
		pbconn:      nil,
		uiAutomator: d,
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
	jsExpr := fmt.Sprintf("document.getElementById('%s')", sku)
	return ClickElementByCDP(ta.pbconn, jsExpr)(ctx)
}

// BuySku clicks the buy button on a Play Billing overlay.
func (ta *TestApp) BuySku(ctx context.Context) error {
	return ClickButtonOnArcPaymentOverlay(ta.uiAutomator, "Button", "Buy")(ctx)
}

// RequiredAuthConfirm clicks yes, always button in required auth window.
func (ta *TestApp) RequiredAuthConfirm(ctx context.Context) error {
	return RequiredAuthConfirm(ta.uiAutomator)(ctx)
}

// CheckPaymentSuccessful checks for a presence of payment successful screen in the android ui tree.
func (ta *TestApp) CheckPaymentSuccessful(ctx context.Context) error {
	return CheckPaymentSuccessful(ta.uiAutomator)(ctx)
}
