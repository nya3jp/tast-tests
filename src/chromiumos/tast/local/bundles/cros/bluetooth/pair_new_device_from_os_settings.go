// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// osSettingsPairNewDeviceModal is the modal that is opened when the "pair new
// device" button within the Bluetooth Settings is pressed.
var osSettingsPairNewDeviceModal = nodewith.NameContaining("Pair new device").Role(role.Heading)

// osSettingsPairNewDeviceButton is the "pair new device" button within the
// Bluetooth Settings.
var osSettingsPairNewDeviceButton = nodewith.NameContaining("Pair new device").Role(role.Button)

func init() {
	testing.AddTest(&testing.Test{
		Func: PairNewDeviceFromOSSettings,
		Desc: "Checks that the pairing dialog can be opened from the OS Settings",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithBluetoothEnabled",
	})
}

// PairNewDeviceFromOSSettings tests that a user can successfully open the
// pairing dialog from the "Pair new device" button on the OS Settings page.
func PairNewDeviceFromOSSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	app, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch OS Settings: ", err)
	}
	defer app.Close(ctx)

	if err := bluetooth.Enable(ctx); err != nil {
		s.Fatal("Failed to enable Bluetooth: ", err)
	}

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Open the \"Pair new device\" dialog",
		ui.LeftClick(osSettingsPairNewDeviceButton),
		ui.WaitUntilExists(osSettingsPairNewDeviceModal),
	)(ctx); err != nil {
		s.Fatal("Failed to open the \"Pair new device\" dialog: ", err)
	}
}
