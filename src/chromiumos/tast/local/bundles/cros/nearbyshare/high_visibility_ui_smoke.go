// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"

	"chromiumos/tast/local/bundles/cros/nearbyshare/nearbysetup"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HighVisibilityUISmoke,
		Desc: "Checks that Nearby Share high-visibility receiving can be initiated from Quick Settings",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
	})
}

// HighVisibilityUISmoke tests that we can open the receiving UI surface from Quick Settings.
func HighVisibilityUISmoke(ctx context.Context, s *testing.State) {
	// TODO(crbug/1159975): Remove flags (or use precondition) once the feature is enabled by default.
	cr, err := chrome.New(
		ctx,
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	const deviceName = "device_HighVisibilityUISmoke"
	if err := nearbysetup.CrOSSetup(ctx, tconn, cr, nearbyshare.DataUsageOnline, nearbyshare.VisibilityAllContacts, deviceName); err != nil {
		s.Fatal("Failed to set up Nearby Share: ", err)
	}
	if err := nearbyshare.StartHighVisibilityMode(ctx, tconn, deviceName); err != nil {
		s.Fatal("Failed to enable Nearby Share's high visibility mode: ", err)
	}
}
