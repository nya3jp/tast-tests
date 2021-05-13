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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UIToggleFromQuicksettings,
		Desc: "Enable and disable Bluetooth from minimized quick setttings UI",
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

func UIToggleFromQuicksettings(ctx context.Context, s *testing.State) {
	systemTray := nodewith.ClassName("UnifiedSystemTray")                                  // System tray element on the menu bar. Clicking on this will bring up the quick setting button
	systemTrayExpandButton := nodewith.ClassName("ExpandButton")                           // Button to expand the quick setting menu
	systemTrayCollapseButton := nodewith.ClassName("CollapseButton")                       // Button to collapse the quick setting menu
	bluetoothTurnOffButton := nodewith.NameContaining("Toggle Bluetooth. Bluetooth is on") // Bluetooth button in the quick setting menu, when Bluetooth is on
	bluetoothTurnOnButton := nodewith.NameContaining("Toggle Bluetooth. Bluetooth is off") // Bluetooth button in the quick setting menu, when Bluetooth is off

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	// Power on Bluetooth adapter
	err = bluetooth.Enable(ctx)
	if err != nil {
		s.Fatal("Failed to power on Bluetooth adapter: ", err)
	}

	if err := uiauto.Combine("bring up quick setting menu and collapse if needed",
		ui.LeftClick(systemTray),
		ui.IfSuccessThen(ui.Gone(systemTrayExpandButton), ui.LeftClick(systemTrayCollapseButton)))(ctx); err != nil {
		s.Fatal("Failed to bring and collapse the quick settings page: ", err)
	}

	numIterations := 20
	for i := 0; i < numIterations; i++ {
		s.Logf("Iteration %d of %d", i, numIterations)

		// Click on Bluetooth UI button and wait for button state to toggle.
		if err := uiauto.Combine("disable Bluetooth and confirm",
			ui.LeftClick(bluetoothTurnOffButton),
			ui.WaitUntilExists(bluetoothTurnOnButton),
		)(ctx); err != nil {
			s.Fatal("Failed to left click the settings bubble: ", err)
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
			s.Fatal("Failed to left click the settings bubble: ", err)
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
