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
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// TODO(crbug.com/1252917): Remove this test when the Bluetooth Revamp has
// fully launched.

func init() {
	testing.AddTest(&testing.Test{
		Func:         UIToggleFromQuicksettings,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Enable and disable Bluetooth from minimized quick setttings UI",
		Contacts: []string{
			"chromeos-bluetooth-champs@google.com", // https://b.corp.google.com/issues/new?component=167317&template=1370210.
			"chromeos-bluetooth-engprod@google.com",
			"shijinabraham@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithBluetoothRevampDisabled",
	})
}

// UIToggleFromQuicksettings tests enabling/disabling Bluetooth from minimized quick settings UI
func UIToggleFromQuicksettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	// Bluetooth button in the quick setting menu, when Bluetooth is on.
	bluetoothTurnOffButton := nodewith.NameContaining("Toggle Bluetooth. Bluetooth is on").Role(role.ToggleButton)
	// Bluetooth button in the quick setting menu, when Bluetooth is off.
	bluetoothTurnOnButton := nodewith.NameContaining("Toggle Bluetooth. Bluetooth is off").Role(role.ToggleButton)

	// Power on Bluetooth adapter
	if err = bluetooth.Enable(ctx); err != nil {
		s.Fatal("Failed to power on Bluetooth adapter: ", err)
	}

	// The Quick Settings is collapsed to avoid being taken to the detailed
	// Bluetooth view when we press the Bluetooth feature pod icon button
	// and it enables Bluetooth.
	if err := quicksettings.Collapse(ctx, tconn); err != nil {
		s.Fatal("Failed to collapse the Quick Settings: ", err)
	}
	defer quicksettings.Expand(ctx, tconn)

	numIterations := 20
	for i := 0; i < numIterations; i++ {
		s.Logf("Iteration %d of %d", i, numIterations)

		// Click on Bluetooth UI button and wait for button state to toggle.
		if err := uiauto.Combine("disable Bluetooth and confirm",
			ui.LeftClick(bluetoothTurnOffButton),
			ui.WaitUntilExists(bluetoothTurnOnButton),
		)(ctx); err != nil {
			s.Fatal("Failed to left click the settings button: ", err)
		}
		// Confirm Bluetooth adapter is disabled.
		status, err := bluetooth.IsEnabled(ctx)
		if err != nil {
			s.Fatal("Failed to check Bluetooth status: ", err)
		}
		if status {
			s.Fatal("Failed to verify Bluetooth status, got enabled, want disabled")
		}

		// Click on Bluetooth UI button and wait for button state to toggle.
		if err := uiauto.Combine("enable Bluetooth and confirm",
			ui.LeftClick(bluetoothTurnOnButton),
			ui.WaitUntilExists(bluetoothTurnOffButton),
		)(ctx); err != nil {
			s.Fatal("Failed to left click the settings button: ", err)
		}
		// Confirm Bluetooth adapter is disabled.
		status, err = bluetooth.IsEnabled(ctx)
		if err != nil {
			s.Fatal("Failed to check Bluetooth status: ", err)
		}
		if !status {
			s.Fatal("Failed to verify Bluetooth status, got disabled, want enabled")
		}

	}
}
