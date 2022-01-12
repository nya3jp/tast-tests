// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/bluetooth/mojo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ToggleBluetoothUsingMojo,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that Bluetooth can be enabled and disabled using Mojo API",
		Contacts: []string{
			"shijinabraham@google.com",
			"cros-conn-test-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "bluetoothMojoJSObject",
	})
}

// ToggleBluetoothUsingMojo toggles Bluetooth state using Bluetooth
// mojo API call and confirm the state change via platform API
// and using state in mojo
func ToggleBluetoothUsingMojo(ctx context.Context, s *testing.State) {
	bluetoothMojo := s.FixtValue().(*mojo.JSObject).Js

	const iterations = 5
	for i := 0; i < iterations; i++ {

		var state bool
		var want mojo.BluetoothSystemState

		if i%2 == 0 {
			state = false
			want = mojo.Disabled
		} else {
			state = true
			want = mojo.Enabled
		}
		s.Logf("(iteration %d of %d)", i+1, iterations)
		s.Logf("Toggling Bluetooth state to %t", state)

		if err := mojo.SetBluetoothEnabledState(ctx, bluetoothMojo, state); err != nil {
			s.Fatal("Failed to toggle Bluetooth state via mojo: ", err)
		}

		if err := bluetooth.PollForAdapterState(ctx, state); err != nil {
			s.Fatal("Bluetooth state not as expected: ", err)
		}

		if err := mojo.PollForBluetoothSystemState(ctx, bluetoothMojo, want); err != nil {
			s.Fatal("Failed to get SystemProperties: ", err)
		}
	}
}
