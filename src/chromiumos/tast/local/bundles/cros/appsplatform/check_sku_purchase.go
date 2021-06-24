// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package appsplatform

import (
	"context"

	"chromiumos/tast/local/playbilling"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckSkuPurchase,
		Desc: "Verify the ARC Payments overlay appears and can be navigated",
		Contacts: []string{
			"benreich@chromium.org",
			"jshikaram@chromium.org",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "playBillingFixture",
		Data:         playbilling.DataFiles,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// CheckSkuPurchase uses the test SKU android.test.purchased to test the purchase flow.
func CheckSkuPurchase(ctx context.Context, s *testing.State) {
	testApp := s.FixtValue().(*playbilling.FixtData).TestApp

	if err := testApp.Launch(ctx); err != nil {
		s.Fatal("Failed to launch Play Billing test app: ", err)
	}

	if err := testApp.OpenBillingDialog(ctx, "android_test_purchased"); err != nil {
		s.Fatal("Failed to find and click the \"Buy Test SKU\" button: ", err)
	}

	// TODO(crbug.com/1223716): creating a uiAutomator here because of the bug.
	arcDevice := s.FixtValue().(*playbilling.FixtData).ARC
	uiAutomator, err := arcDevice.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}

	if err := testApp.BuySku(ctx, uiAutomator); err != nil {
		s.Fatal("Failed to click the buy button on the billing dialog: ", err)
	}

	if err := testApp.ClickButtonOnArcPaymentOverlay(ctx, uiAutomator, "RadioButton", "Yes, always"); err != nil {
		s.Fatal("Failed clicking Yes, always radio button: ", err)
	}

	// Click the ok button after always has been selected.
	if err := testApp.ClickButtonOnArcPaymentOverlay(ctx, uiAutomator, "Button", "OK"); err != nil {
		s.Fatal("Failed clicking OK button: ", err)
	}

	if err := testApp.CheckPresenceOfArcObject(ctx, uiAutomator, "android.widget.TextView", "Payment successful"); err != nil {
		s.Fatal("Payment successful did not appear: ", err)
	}
}
