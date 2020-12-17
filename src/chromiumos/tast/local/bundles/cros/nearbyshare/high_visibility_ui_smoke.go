// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/local/bundles/cros/nearbyshare/nearbysetup"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/quicksettings"
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

	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open Quick Settings: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer quicksettings.Hide(ctx, tconn)

	if err := quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodNearbyShare, true); err != nil {
		s.Fatal("Failed to enter Nearby Share high-visibility mode: ", err)
	}

	receiveWindow, err := ui.FindWithTimeout(ctx, tconn, nearbyshare.ReceiveUIParams, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find Nearby Share receiving window: ", err)
	}
	defer receiveWindow.Release(ctx)

	// Check for the text in the dialog that shows the displayed device name and that we're visible to nearby devices.
	// This text includes a dynamic countdown for remaining high-visibility time that changes dynamically, so we'll match a substring.
	r, err := regexp.Compile(fmt.Sprintf("Visible to nearby devices as %v", deviceName))
	if err != nil {
		s.Fatal("Failed to compile regexp for visibility and device name text: ", err)
	}
	textParams := ui.FindParams{
		Role:       ui.RoleTypeStaticText,
		Attributes: map[string]interface{}{"name": r},
	}
	if err := receiveWindow.WaitUntilDescendantExists(ctx, textParams, 10*time.Second); err != nil {
		s.Fatal("Failed to find text with device name and visibility indication: ", err)
	}
}
