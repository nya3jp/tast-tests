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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: EnableDisableBluetooth,
		Desc: "Enable and disable Bluetooth from minimized quick setttings UI ",
		Contacts: []string{
			"chromeos-bluetooth-champs@google.com", // https://b.corp.google.com/issues/new?component=167317&template=1370210
			"chromeos-bluetooth-engprod@google.com",
			"shijinabraham@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func EnableDisableBluetooth(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	/*
		// Initialize gRPC connection with DUT.
		d := s.DUT()
		r, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect rpc: ", err)
		}
		defer r.Close(ctx)

		// Enable Bluetooth
		btClient := network.NewBluetoothServiceClient(r.Conn)
		if _, err := btClient.SetBluetoothPoweredFast(ctx, &network.SetBluetoothPoweredFastRequest{Powered: true}); err != nil {
			s.Error("Could not enable Bluetooth: ", err)
		}

	*/
	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Bring up the quick setting menu",
		ui.LeftClick(nodewith.ClassName("UnifiedSystemTray")),
	)(ctx); err != nil {
		s.Fatal("Failed to bring up the quicksettings menu: ", err)
	}

	// Check if quick settings page is not collapsed
	if err := ui.Exists(nodewith.ClassName("ExpandButton"))(ctx); err != nil {
		// Collaspe the quick settings page
		if err := ui.LeftClick(nodewith.ClassName("CollapseButton"))(ctx); err != nil {
			s.Fatal("Failed to collapse quick settings page: ", err)
		}
	}

	numIterations := 20
	for i := 0; i < numIterations; i++ {
		s.Logf("Iteration %d of %d", i, numIterations)

		// Click on Bluetooth UI button and wait for button state to toggle
		if err := uiauto.Combine("Disable Bluetooth and confirm",
			ui.LeftClick(nodewith.NameContaining("Toggle Bluetooth. Bluetooth is on")),
			ui.WaitUntilExists(nodewith.NameContaining("Toggle Bluetooth. Bluetooth is off")),
		)(ctx); err != nil {
			s.Fatal("Failed to left click the settings bubble: ", err)
		}
		// Confirm Bluetooth adapter is disabled
		status, err := bluetooth.IsEnabled(ctx)
		if err != nil {
			s.Fatal("Error while checking Bluetooth status: ", err)
		}
		if status {
			s.Fatal("Bluetooth not disabled")
		}

		// Click on Bluetooth UI button and wait for button state to toggle
		if err := uiauto.Combine("Disable Bluetooth and confirm",
			ui.LeftClick(nodewith.NameContaining("Toggle Bluetooth. Bluetooth is off")),
			ui.WaitUntilExists(nodewith.NameContaining("Toggle Bluetooth. Bluetooth is on")),
		)(ctx); err != nil {
			s.Fatal("Failed to left click the settings bubble: ", err)
		}
		// Confirm Bluetooth adapter is disabled
		status, err = bluetooth.IsEnabled(ctx)
		if err != nil {
			s.Fatal("Error while checking Bluetooth status: ", err)
		}
		if !status {
			s.Fatal("Bluetooth not enabled")
		}

	}
}
