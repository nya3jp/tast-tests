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
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UIToggleFromBTQuicksettings,
		Desc: "Enable and disable Bluetooth from quick setttings Bluetooth UI",
		Contacts: []string{
			"chromeos-bluetooth-champs@google.com", // https://b.corp.google.com/issues/new?component=167317&template=1370210.
			"chromeos-bluetooth-engprod@google.com",
			"shijinabraham@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// UIToggleFromBTQuicksettings tests enabling/disabling Bluetooth from maximized quick settings UI.
// Enabling Bluetooth in maximized quick settings UI opens the Bluetooth quick settings UI.
// Since there is not UI element to wait on in Bluetooth quick settings UI, we need to poll the Adapter power state to avoid timing issues.
func UIToggleFromBTQuicksettings(ctx context.Context, s *testing.State) {

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	// TODO(b/188767517): Add unique identifiers to UI elements used in these tests
	// System tray element on the menu bar. Clicking on this will bring up the quick setting button.
	systemTray := nodewith.ClassName("UnifiedSystemTray").Role(role.Button)
	// Button to expand the quick setting menu.
	systemTrayExpandButton := nodewith.ClassName("ExpandButton").Role(role.Button)
	// Button to collapse the quick setting menu.
	systemTrayCollapseButton := nodewith.ClassName("CollapseButton").Role(role.Button)
	// Bluetooth button in the quick setting menu, when Bluetooth is on.
	bluetoothTurnOffButton := nodewith.NameContaining("Toggle Bluetooth. Bluetooth is on").ClassName("FeaturePodIconButton").Role(role.ToggleButton)
	// Bluetooth button in the quick setting menu, when Bluetooth is off.
	bluetoothTurnOnButton := nodewith.NameContaining("Toggle Bluetooth. Bluetooth is off").ClassName("FeaturePodIconButton").Role(role.ToggleButton)
	// Bluetooth Quick Settings UI.
	bluetoothSettings := nodewith.ClassName("BluetoothDetailedView")
	// Bluetooth button in the bluetooth quick setting menu, when Bluetooth is off.
	bluetoothToggleButton := nodewith.Name("Bluetooth").ClassName("ToggleButton").Role(role.Switch)

	// Power on Bluetooth adapter.
	if err = bluetooth.Enable(ctx); err != nil {
		s.Fatal("Failed to power on Bluetooth adapter: ", err)
	}

	if err := uiauto.Combine("bring up quick setting menu and expand if needed",
		ui.LeftClick(systemTray),
		ui.IfSuccessThen(ui.Gone(systemTrayCollapseButton), ui.LeftClick(systemTrayExpandButton)))(ctx); err != nil {
		s.Fatal("Failed to bring up and expand the quick settings page: ", err)
	}

	// Click on Bluetooth UI button and wait for button state to toggle.
	// Enabling Bluetooth from quick setting menu should bring up the bluetooth quick setting screen.
	if err := uiauto.Combine("disable/enable bluetooth and confirm Bluetooth quick setting menu is present ",
		ui.WaitUntilExists(systemTrayCollapseButton),
		ui.LeftClick(bluetoothTurnOffButton),
		ui.WaitUntilExists(bluetoothTurnOnButton),
		ui.LeftClick(bluetoothTurnOnButton),
		ui.WaitUntilExists(bluetoothSettings),
	)(ctx); err != nil {
		s.Fatal("Failed to bring up Bluetooth quick settings UI: ", err)
	}

	if err = bluetooth.PollForBTEnabled(ctx); err != nil {
		s.Fatal("Failed to verify Bluetooth status, got disabled, want enabled: ", err)
	}

	// Confirm Bluetooth adapter is enabled.
	status, err := bluetooth.IsEnabled(ctx)
	if err != nil {
		s.Fatal("Failed to check Bluetooth status: ", err)
	}
	if !status {
		s.Fatal("Failed to verify Bluetooth status, got disabled, want enabled: ", err)
	}

	const numIterations = 20
	for i := 0; i < numIterations; i++ {
		s.Logf("Iteration %d of %d", i, numIterations)

		// Click on Bluetooth UI button and wait for button state to toggle.
		if err := uiauto.Combine("disable Bluetooth and confirm",
			ui.LeftClick(bluetoothToggleButton),
		)(ctx); err != nil {
			s.Fatal("Failed to disable Bluetooth via toggle button: ", err)
		}

		// Confirm Bluetooth adapter is disabled.
		if err = bluetooth.PollForBTDisabled(ctx); err != nil {
			s.Fatal("Failed to verify Bluetooth status, got enabled, want disabled: ", err)
		}

		// Click on Bluetooth UI button and wait for button state to toggle.
		if err := uiauto.Combine("enable Bluetooth and confirm",
			ui.LeftClick(bluetoothToggleButton),
		)(ctx); err != nil {
			s.Fatal("Failed to enable Bluetooth via toggle button: ", err)
		}

		// Confirm Bluetooth adapter is disabled.
		if err = bluetooth.PollForBTEnabled(ctx); err != nil {
			s.Fatal("Failed to verify Bluetooth status, got disabled, want enabled: ", err)
		}
	}
}
