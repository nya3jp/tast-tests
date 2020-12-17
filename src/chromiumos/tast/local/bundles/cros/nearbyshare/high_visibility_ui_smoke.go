// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/nearbyshare/nearbysetup"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
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
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := nearbysetup.CrOSSetup(ctx, tconn, cr, nearbyshare.CrOSDataUsageOnline, nearbyshare.CrOSVisibilityAllContacts, ""); err != nil {
		s.Fatal("Failed to set up Nearby Share: ", err)
	}

	receiveUI, err := nearbyshare.EnterHighVisibility(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to enter high-visibility mode: ", err)
	}
	defer receiveUI.Release(ctx)

	// Check for the cancel button in the UI.
	if err := receiveUI.Root.WaitUntilDescendantExists(ctx, ui.FindParams{Role: ui.RoleTypeButton, Name: "Cancel"}, 10*time.Second); err != nil {
		s.Fatal("Failed to find receiving dialog's Cancel button: ", err)
	}
}
