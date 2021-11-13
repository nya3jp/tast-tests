// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/local/bundles/cros/bluetooth/bluetoothutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OpenBluetoothSettingsFromQuickSettings,
		Desc: "Checks that clicking the Settings button on the detailed Bluetooth page within the Quick Settings navigates to the Bluetooth Settings",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithBluetoothRevamp",
	})
}

// OpenBluetoothSettingsFromQuickSettings tests that a user can successfully
// navigate through to the Bluetooth sub-page within the OS Settings from the
// Settings button in the Bluetooth detailed view within the Quick Settings.
func OpenBluetoothSettingsFromQuickSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := bluetoothutil.ShowDetailedView(ctx, tconn); err != nil {
		s.Fatal("Failed to show the detailed Bluetooth view: ", err)
	}

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Click the Bluetooth Settings button",
		ui.WaitUntilExists(bluetoothutil.DetailedViewSettingsButton),
		ui.LeftClick(bluetoothutil.DetailedViewSettingsButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click the Bluetooth Settings button: ", err)
	}

	// Check if the Bluetooth sub-page within the OS Settings was opened.
	matcher := chrome.MatchTargetURL(bluetoothutil.BluetoothSubPageURL)
	if _, err := cr.NewConnForTarget(ctx, matcher); err != nil {
		s.Fatal("Failed to open the Bluetooth settings: ", err)
	}
}
