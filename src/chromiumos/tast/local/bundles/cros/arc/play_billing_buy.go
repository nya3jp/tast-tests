// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"github.com/mafredri/cdp/protocol/input"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PlayBillingBuy,
		Desc: "Sideload test Play Billing app and make a purchase",
		Contacts: []string{
			"benreich@chromium.org",
			"jshikaram@chromium.org",
			"chromeos-apps-foundation-team@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Timeout:      7 * time.Minute,
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Data: []string{"dev.conn.drink.apk"},
		Vars: []string{
			"arc.PlayBillingBuy.username",
			"arc.PlayBillingBuy.password",
		},
	})
}

func PlayBillingBuy(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("arc.PlayBillingBuy.username")
	password := s.RequiredVar("arc.PlayBillingBuy.password")

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.ARCSupported(),
		chrome.ExtraArgs("--enable-experimental-web-platform-features"),
		chrome.EnableFeatures("AppStoreBillingDebug", "WebPaymentsExperimentalFeatures"),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// Setup Test API Connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}

	// Setup ARC device and UI Automator.
	arcDevice, uiAutomator, err := setUpARCPlayBilling(ctx, cr, s.OutDir())
	if err != nil {
		s.Fatal("Failed to setup ARC: ", err)
	}
	defer arcDevice.Close(cleanupCtx)
	defer uiAutomator.Close(cleanupCtx)

	// Sideloar the dev.conn.drink APK.
	if err := arcDevice.Install(ctx, s.DataPath("dev.conn.drink.apk")); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	// Launch the dev.conn.drink via App Service.
	if err := apps.Launch(ctx, tconn, "hbamklmgbeefohddhpaagjeaggnfhcmd"); err != nil {
		s.Fatal("Failed launching the PWA of dev.conn.drink: ", err)
	}

	// Attach to the dev.conn.drink app to send CDP commands.
	pbconn, err := cr.NewConnForTarget(ctx, func(t *chrome.Target) bool {
		return t.URL == "https://beer.conn.dev/index.html"
	})
	if err != nil {
		s.Fatal("Failed to setup connection to play billing PWA: ", err)
	}

	// Wait for the Buy button to exist and stabilize it's location.
	jsExpr := "document.getElementById('pay_billing')"
	if err := pbconn.WaitForExprFailOnErrWithTimeout(ctx, fmt.Sprintf("%s != undefined", jsExpr), 30*time.Second); err != nil {
		s.Fatal("Failed to wait for the play billing button to appear: ", err)
	}
	type DOMRect struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	var previousLocation, currentLocation DOMRect
	start := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := pbconn.Eval(ctx, fmt.Sprintf(`new Promise(resolve => {
			const domRect = %s.getBoundingClientRect();
			resolve({
				x: domRect.x,
				y: domRect.y,
			});
		})`, jsExpr), &currentLocation); err != nil {
			previousLocation = DOMRect{}
			return err
		}
		if currentLocation != previousLocation {
			previousLocation = currentLocation
			elapsed := time.Since(start)
			return errors.Errorf("node has not stopped changing location after %s, perhaps increase timeout or use ImmediateLocation", elapsed)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
		s.Fatal("Failed to wait for buy button to stabilize: ", err)
	}

	// Dispatch a CDP mouse event to click and release relative to the PWA window.
	mousePressed := input.NewDispatchMouseEventArgs("mousePressed", currentLocation.X, currentLocation.Y).SetClickCount(1).SetButton(input.MouseButtonLeft)
	if err := pbconn.DispatchMouseEvent(ctx, mousePressed); err != nil {
		s.Fatalf("Failed to click left mouse button at %v: %v", currentLocation, err)
	}
	mousePressed.Type = "mouseReleased"
	if err := pbconn.DispatchMouseEvent(ctx, mousePressed); err != nil {
		s.Fatalf("Failed to release left mouse button at %v: %v", currentLocation, err)
	}

	// Wait for the buy button to appear on the ARC Payments overlay then click it.
	if err := clickButtonOnARCPaymentOverlay(ctx, uiAutomator, "Button", "Buy"); err != nil {
		s.Fatal("Failed clicking Buy button: ", err)
	}

	// Ensure authentication requires for all purchases is clicked.
	if err := clickButtonOnARCPaymentOverlay(ctx, uiAutomator, "RadioButton", "Yes, always"); err != nil {
		s.Fatal("Failed clicking Yes, always radio button: ", err)
	}

	// Click the ok button after always has been selected.
	if err := clickButtonOnARCPaymentOverlay(ctx, uiAutomator, "Button", "OK"); err != nil {
		s.Fatal("Failed clicking OK button: ", err)
	}

	paymentSuccessOverlay := uiAutomator.Object(ui.ClassName("android.widget.TextView"), ui.TextMatches("Payment successful"), ui.Enabled(true))
	if err := paymentSuccessOverlay.WaitForExists(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed waiting 10s for the payment successful overlay to exist: ", err)
	}
}

func clickButtonOnARCPaymentOverlay(ctx context.Context, uiAutomator *ui.Device, buttonType, objectText string) error {
	button := uiAutomator.Object(ui.ClassName("android.widget."+buttonType), ui.TextMatches(objectText), ui.Enabled(true))
	if err := button.WaitForExists(ctx, 10*time.Second); err != nil {
		return errors.Wrapf(err, "failed waiting %v for button to exist", 10*time.Second)
	}
	return button.Click(ctx)
}

// setUpARCPlayBilling starts an ARC device and starts UI automator.
func setUpARCPlayBilling(ctx context.Context, cr *chrome.Chrome, outDir string) (*arc.ARC, *ui.Device, error) {
	// Setup ARC device.
	arcDevice, err := arc.New(ctx, outDir)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start ARC")
	}

	// Start up UI automator.
	uiAutomator, err := arcDevice.NewUIDevice(ctx)
	if err != nil {
		if err := arcDevice.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close UI automator: ", err)
		}
		return nil, nil, errors.Wrap(err, "failed to initialize UI automator")
	}

	if err := arcDevice.WaitIntentHelper(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to wait for intent helper")
	}

	return arcDevice, uiAutomator, nil
}
