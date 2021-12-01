// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
)

// bluetoothSettingsPairNewDeviceModal is the modal that is opened when the
// "pair new device" button within the Bluetooth Settings is pressed.
var bluetoothSettingsPairNewDeviceModal = nodewith.NameContaining("Pair new device").Role(role.Heading)

// bluetoothSettingsPairNewDeviceButton is the "pair new device" button within
// the Bluetooth Settings.
var bluetoothSettingsPairNewDeviceButton = nodewith.NameContaining("Pair new device").Role(role.Button)

func init() {
	testing.AddTest(&testing.Test{
		Func: PairNewDeviceFromBluetoothSettings,
		Desc: "Checks that the pairing dialog can be opened from the Bluetooth Settings sub-page",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithBluetoothEnabled",
	})
}

// PairNewDeviceFromBluetoothSettings tests that a user can successfully open
// the pairing dialog from the "Pair new device" button on the Bluetooth
// Settings sub-page.
func PairNewDeviceFromBluetoothSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	app, err := ossettings.NavigateToBluetoothSettingsPage(ctx, s, tconn)
	defer app.Close(ctx)

	if err != nil {
		s.Fatal("Failed to show the Bluetooth Settings sub-page: ", err)
	}

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Open the \"Pair new device\" dialog",
		ui.LeftClick(bluetoothSettingsPairNewDeviceButton),
		ui.WaitUntilExists(bluetoothSettingsPairNewDeviceModal),
	)(ctx); err != nil {
		s.Fatal("Failed to open the \"Pair new device\" dialog: ", err)
	}
}
