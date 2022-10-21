// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/local/bluetooth/bluez"
	"chromiumos/tast/local/bluetooth/mojo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ToggleBluetoothUsingMojo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Bluetooth can be enabled and disabled using Mojo API",
		Contacts: []string{
			"shijinabraham@google.com",
			"cros-conn-test-team@google.com",
		},
		Attr:         []string{"group:bluetooth", "bluetooth_flaky"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "bluetoothMojoJSObject",
	})
}

// ToggleBluetoothUsingMojo toggles Bluetooth state using Bluetooth
// mojo API call and confirm the state change via platform API
// and using state in mojo
func ToggleBluetoothUsingMojo(ctx context.Context, s *testing.State) {
	bluetoothMojo := s.FixtValue().(*mojo.BTConn).Js

	const iterations = 5
	for i := 0; i < iterations; i++ {

		var isEnabled bool
		var expectedState mojo.BluetoothSystemState

		if i%2 == 0 {
			isEnabled = false
			expectedState = mojo.Disabled
		} else {
			isEnabled = true
			expectedState = mojo.Enabled
		}

		s.Logf("Toggling Bluetooth state to %t (iteration %d of %d)", isEnabled, i+1, iterations)

		if err := mojo.SetBluetoothEnabledState(ctx, *bluetoothMojo, isEnabled); err != nil {
			s.Fatal("Failed to toggle Bluetooth state via mojo: ", err)
		}

		if err := bluez.PollForAdapterState(ctx, isEnabled); err != nil {
			s.Fatal("Bluetooth state not as expected: ", err)
		}

		if err := mojo.PollForBluetoothSystemState(ctx, *bluetoothMojo, expectedState); err != nil {
			s.Fatal("Failed to get SystemProperties: ", err)
		}
	}
}
