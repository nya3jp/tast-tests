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

// TODO(crbug.com/1252917): Remove this test when the Bluetooth Revamp has
// fully launched.

func init() {
	testing.AddTest(&testing.Test{
		Func:         UIToggleFromBTSettings,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Enable and disable Bluetooth from Bluetooth Settings UI",
		Contacts: []string{
			"chromeos-bluetooth-champs@google.com", // b/new?component=167317&template=1370210.
			"chromeos-bluetooth-engprod@google.com",
			"shijinabraham@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithBluetoothRevampDisabled",
	})
}

// UIToggleFromBTSettings tests enabling/disabling Bluetooth from the Bluetooth settings UI in ChromeOS settings.
func UIToggleFromBTSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	// Bluetooth button in the ChromeOS settings.
	bluetoothToggleButton := nodewith.Name("Bluetooth enable").Role(role.ToggleButton)
	// URL for Bluetooth page in ChromeOS settings menu
	bluetoothSettingsURL := "bluetoothDevices"

	// Power on Bluetooth adapter.
	if err = bluetooth.Enable(ctx); err != nil {
		s.Fatal("Failed to power on Bluetooth adapter: ", err)
	}

	// Launch the ChromeOS settings application.
	if _, err = ossettings.LaunchAtPageURL(ctx, tconn, cr, bluetoothSettingsURL, ui.Exists(bluetoothToggleButton)); err != nil {
		s.Fatal("Failed to bring up Bluetooth os settings page: ", err)
	}

	const numIterations = 20
	for i := 0; i < numIterations; i++ {
		s.Logf("Iteration %d of %d", i, numIterations)

		// Click on Bluetooth UI button and wait for button state to toggle.
		if err := ui.LeftClick(bluetoothToggleButton)(ctx); err != nil {
			s.Fatal("Failed to left click bluetooth toggle button: ", err)
		}

		// Confirm Bluetooth adapter is disabled.
		if err = bluetooth.PollForBTDisabled(ctx); err != nil {
			s.Fatal("Failed to verify Bluetooth status, got enabled, want disabled: ", err)
		}

		// Click on Bluetooth UI button and wait for button state to toggle.
		if err := ui.LeftClick(bluetoothToggleButton)(ctx); err != nil {
			s.Fatal("Failed to left click the bluetooth toggle button: ", err)
		}

		// Confirm Bluetooth adapter is enabled.
		if err = bluetooth.PollForBTEnabled(ctx); err != nil {
			s.Fatal("Failed to verify Bluetooth status, got disabled, want enabled: ", err)
		}
	}
}
