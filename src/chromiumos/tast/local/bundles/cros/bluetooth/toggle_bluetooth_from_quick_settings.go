// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/local/bluetooth/bluez"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ToggleBluetoothFromQuickSettings,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that Bluetooth can be enabled and disabled from within the Quick Settings",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithBluetoothEnabled",
	})
}

// ToggleBluetoothFromQuickSettings tests that a user can successfully toggle
// the Bluetooth state using the Bluetooth feature pod icon button within the
// Quick Settings.
func ToggleBluetoothFromQuickSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// The Quick Settings is collapsed to avoid being taken to the detailed
	// Bluetooth view when we press the Bluetooth feature pod icon button
	// and it enables Bluetooth.
	if err := quicksettings.Collapse(ctx, tconn); err != nil {
		s.Fatal("Failed to collapse the Quick Settings: ", err)
	}
	defer quicksettings.Expand(ctx, tconn)

	if err := bluez.Enable(ctx); err != nil {
		s.Fatal("Failed to enable Bluetooth: ", err)
	}

	ui := uiauto.New(tconn)

	state := false
	const iterations = 20
	for i := 0; i < iterations; i++ {
		s.Logf("Toggling Bluetooth (iteration %d of %d)", i+1, iterations)

		if err := ui.LeftClick(quicksettings.PodIconButton(quicksettings.SettingPodBluetooth))(ctx); err != nil {
			s.Fatal("Failed to click the Bluetooth feature pod icon button: ", err)
		}
		if err := bluez.PollForAdapterState(ctx, state); err != nil {
			s.Fatal("Failed to toggle Bluetooth state: ", err)
		}
		state = !state
	}
}
