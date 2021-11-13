// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/bundles/cros/bluetooth/bluetoothutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PairNewDeviceFromBluetoothQuickSettings,
		Desc: "Checks that the pairing dialog can be opened from within the Bluetooth Quick Settings",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithBluetoothRevamp",
	})
}

// PairNewDeviceFromBluetoothQuickSettings tests that a user can successfully
// open the pairing dialog from the "Pair new device" button in the detailed
// Bluetooth view within the Quick Settings.
func PairNewDeviceFromBluetoothQuickSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := bluetoothutil.ShowDetailedView(ctx, tconn); err != nil {
		s.Fatal("Failed to show the detailed Bluetooth view: ", err)
	}

	if err := bluetooth.PollForBTEnabled(ctx); err != nil {
		s.Fatal("Expected Bluetooth to be enabled: ", err)
	}

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Click the \"Pair new device\" button",
		ui.WaitUntilExists(bluetoothutil.DetailedViewPairNewDeviceButton),
		ui.LeftClick(bluetoothutil.DetailedViewPairNewDeviceButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click the \"Pair new device\" button: ", err)
	}

	// Check if the Bluetooth pairing dialog was opened.
	matcher := chrome.MatchTargetURL(bluetoothutil.BluetoothPairingDialogURL)
	if _, err := cr.NewConnForTarget(ctx, matcher); err != nil {
		s.Fatal("Failed to open the Bluetooth pairing dialog: ", err)
	}
}
