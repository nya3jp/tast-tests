// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package appsplatform

import (
	"context"
	"time"

	"chromiumos/tast/local/playbilling"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CheckSkuPurchase,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verify the ARC Payments overlay appears and can be navigated",
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
		Timeout: 4 * time.Minute,
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

	if err := testApp.BuySku(ctx); err != nil {
		s.Fatal("Failed to click the buy button on the billing dialog: ", err)
	}

	// Successful payment and required auth(if rendered) screens are rendered together.
	// Successful payment will disappear soon after being rendered.
	// Need to check for successful payment presence first, because if required auth is
	// not rendered, we wait for it. In this case by the time we check for successful
	// payment, the screen will disappear.
	if err := testApp.CheckPaymentSuccessful(ctx); err != nil {
		s.Fatal("Failed to find Payment successful: ", err)
	}

	if err := testApp.RequiredAuthConfirm(ctx); err != nil {
		s.Fatal("Failed to confirm required auth: ", err)
	}
}
