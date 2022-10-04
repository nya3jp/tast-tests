// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

// bluetoothPairingDialogURL is the URL of the Bluetooth pairing dialog.
const bluetoothPairingDialogURL = "chrome://bluetooth-pairing/"

func init() {
	testing.AddTest(&testing.Test{
		Func:         PairNewDeviceFromBluetoothQuickSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that the pairing dialog can be opened from within the Bluetooth Quick Settings",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:    "floss_disabled",
			Fixture: "bluetoothEnabledWithBlueZ",
		}},
	})
}

// PairNewDeviceFromBluetoothQuickSettings tests that a user can successfully
// open the pairing dialog from the "Pair new device" button in the detailed
// Bluetooth view within the Quick Settings.
func PairNewDeviceFromBluetoothQuickSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*bluetooth.ChromeLoggedInWithBluetoothEnabled).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := quicksettings.NavigateToBluetoothDetailedView(ctx, tconn); err != nil {
		s.Fatal("Failed to navigate to the detailed Bluetooth view: ", err)
	}

	bt := s.FixtValue().(*bluetooth.ChromeLoggedInWithBluetoothEnabled).Impl

	if err := bt.PollForEnabled(ctx); err != nil {
		s.Fatal("Expected Bluetooth to be enabled: ", err)
	}

	if err := uiauto.New(tconn).LeftClick(quicksettings.BluetoothDetailedViewPairNewDeviceButton)(ctx); err != nil {
		s.Fatal("Failed to click the \"Pair new device\" button: ", err)
	}

	// Check if the Bluetooth pairing dialog was opened.
	matcher := chrome.MatchTargetURL(bluetoothPairingDialogURL)
	conn, err := cr.NewConnForTarget(ctx, matcher)
	if err != nil {
		s.Fatal("Failed to open the Bluetooth pairing dialog: ", err)
	}
	defer conn.Close()
}
