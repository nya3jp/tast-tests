// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetoothmojo

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"context"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BluetoothToggleUsingMojo,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that Bluetooth can be enabled and disabled using Mojo API",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "bluetoothMojoJSObject",
	})
}

// BluetoothToggleUsingMojo toggles Bluetooth state using Bluetooth
// mojo API call and confirm the state change via platform API
func BluetoothToggleUsingMojo(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	_, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	//Open OS settings App Bluetooth Subpage
	url := "chrome://os-settings/bluetooth"
	crconn, err := apps.LaunchOSSettings(ctx, cr, url)
	if err != nil {
		s.Fatal("Failed to open settings app ", err)
	}

	var bluetoothMojo chrome.JSObject

	if err := crconn.Call(ctx, &bluetoothMojo, BTJS2); err != nil {
		s.Fatal(errors.Wrap(err, "failed to create Bluetooth  mojo JS object"))
	}

	state := false
	const iterations = 5
	for i := 0; i < iterations; i++ {
		s.Logf("(iteration %d of %d)", i+1, iterations)
		s.Logf("Toggling Bluetooth state to %t", state)

		if err := SetBluetoothEnabledState(bluetoothMojo, ctx, s, state); err != nil {
			s.Fatal("Failed to toggle Bluetooth state via mojo: ", err)
		}

		if err := bluetooth.PollForAdapterState(ctx, state); err != nil {
			s.Fatal("Bluetooth state not as expected: ", err)

		}
		state = !state
	}

}
