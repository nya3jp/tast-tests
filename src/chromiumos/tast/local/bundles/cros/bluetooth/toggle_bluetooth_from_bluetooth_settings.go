// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// bluetoothSettingsBluetoothToggleButton is the Bluetooth toggle within the Bluetooth Settings page.
var bluetoothSettingsBluetoothToggleButton = nodewith.NameContaining("Bluetooth").Role(role.ToggleButton)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ToggleBluetoothFromBluetoothSettings,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that Bluetooth can be enabled and disabled from the Bluetooth Settings sub-page",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithBluetoothEnabled",
	})
}

// ToggleBluetoothFromBluetoothSettings tests that a user can successfully
// toggle the Bluetooth state using the Bluetooth toggle within the
// Bluetooth Settings sub-page.
func ToggleBluetoothFromBluetoothSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// TODO(b/216303490): Remove once test pass rate is >99%.
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	app, err := ossettings.NavigateToBluetoothSettingsPage(ctx, tconn)
	defer app.Close(ctx)

	if err != nil {
		s.Fatal("Failed to show the Bluetooth Settings sub-page: ", err)
	}

	ui := uiauto.New(tconn)

	if err := ui.WaitUntilExists(bluetoothSettingsBluetoothToggleButton)(ctx); err != nil {
		s.Fatal("Failed to find the Bluetooth toggle: ", err)
	}

	state := false
	const iterations = 20
	for i := 0; i < iterations; i++ {
		s.Logf("Toggling Bluetooth (iteration %d of %d)", i+1, iterations)

		if err := ui.LeftClick(bluetoothSettingsBluetoothToggleButton)(ctx); err != nil {
			s.Fatal("Failed to click the Bluetooth toggle: ", err)
		}
		if err := bluetooth.PollForAdapterState(ctx, state); err != nil {
			s.Fatal("Failed to toggle Bluetooth state: ", err)
		}
		state = !state
	}
}
