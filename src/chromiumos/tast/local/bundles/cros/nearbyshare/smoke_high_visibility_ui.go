// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"

	"chromiumos/tast/common/cros/nearbyshare/nearbysetup"
	"chromiumos/tast/common/cros/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SmokeHighVisibilityUI,
		Desc: "Checks that Nearby Share high-visibility receiving can be initiated from Quick Settings",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
	})
}

// SmokeHighVisibilityUI tests that we can open the receiving UI surface from Quick Settings.
func SmokeHighVisibilityUI(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
		chrome.ExtraArgs("--nearby-share-verbose-logging"),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Set up Nearby Share on the CrOS device.
	const crosBaseName = "cros_test"
	crosDisplayName := nearbytestutils.RandomDeviceName(crosBaseName)
	if err := nearbysetup.CrOSSetup(ctx, tconn, cr, nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, crosDisplayName); err != nil {
		s.Fatal("Failed to set up Nearby Share: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	if err := nearbyshare.StartHighVisibilityMode(ctx, tconn, crosDisplayName); err != nil {
		s.Fatal("Failed to enable Nearby Share's high visibility mode: ", err)
	}
}
