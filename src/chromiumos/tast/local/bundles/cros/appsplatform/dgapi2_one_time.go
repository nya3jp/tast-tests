// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package appsplatform

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/playbilling/dgapi2"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dgapi2OneTime,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify it is possible to go through a one-time purchase flow in the DGAPI2 test app",
		Contacts: []string{
			"jshikaram@chromium.org",
			"ashpakov@google.com", // until Sept 2022
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "playBillingDgapi2Fixture",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 5 * time.Minute,
	})
}

// Dgapi2OneTime Checks DGAPI2 test app allows to purchase a onetime sku.
func Dgapi2OneTime(ctx context.Context, s *testing.State) {
	p := s.FixtValue().(*dgapi2.FixtDgapiData)
	cr := p.Chrome
	testApp := p.TestApp

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "Dgapi2OneTime")

	// As the tested app is stateful, we might observe purchases from the previously failed test runs.
	// Need to consume them, if available, before we proceed with the test.
	if err := testApp.TryConsumeOneTime(ctx); err != nil {
		s.Fatal("Failed to consume a onetime sku: ", err)
	}

	if err := testApp.PurchaseOneTime(ctx); err != nil {
		s.Fatal("Failed to purchase a onetime sku: ", err)
	}
}
