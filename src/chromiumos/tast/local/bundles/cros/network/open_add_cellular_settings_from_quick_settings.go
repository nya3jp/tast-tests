// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// addCellularURL is the URL of the Bluetooth sub-page within the OS Settings.
const addCellularURL = "chrome://os-settings/networks?type=Cellular&showCellularSetup=true"

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenAddCellularSettingsFromQuickSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the add eSIM profile via activation code flow in the success and failure cases",
		Contacts: []string{
			"hsuregan@google.com",
			"cros-connectivity@google.com@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable"},
		Fixture:      "cellular",
		Timeout:      9 * time.Minute,
	})
}

// OpenAddCellularSettingsFromQuickSettings tests that a user can successfully
// navigate to the Cellular sub-page with the Add Cellular dialog open within
// OS Settings from the Add Cellular button in the Network detailed view within
// Quick Settings.
func OpenAddCellularSettingsFromQuickSettings(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := quicksettings.NavigateToNetworkDetailedView(ctx, tconn, false); err != nil {
		s.Fatal("Failed to navigate to the detailed Bluetooth view: ", err)
	}

	ui := uiauto.New(tconn)
	if err := ui.LeftClick(quicksettings.AddCellularButton)(ctx); err != nil {
		s.Fatal("Did not click Add cellular button: ", err)
	}

	// Check if the Add cellular sub-page within the OS Settings was opened.
	matcher := chrome.MatchTargetURL(addCellularURL)
	conn, err := cr.NewConnForTarget(ctx, matcher)
	if err != nil {
		s.Fatal("Failed to open the Add Cellular dialog: ", err)
	}

	var networkAddedText = nodewith.NameContaining("Set up new network").Role(role.StaticText)
	var lookingForPendingProfilesText = nodewith.NameContaining("Looking for available profiles").Role(role.StaticText)
	if err := uiauto.Combine("Check that add cellular dialog appears",
		ui.WithTimeout(5*time.Second).WaitUntilExists(lookingForPendingProfilesText),
		ui.WithTimeout(5*time.Second).WaitUntilExists(networkAddedText),
	)(ctx); err != nil {
		s.Fatal("Failed to find add cellular dialog: ", err)
	}

	defer conn.Close()
}
