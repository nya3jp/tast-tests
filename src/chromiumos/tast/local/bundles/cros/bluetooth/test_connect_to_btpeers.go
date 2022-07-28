// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestConnectToBTPeers,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that a test can connect to btpeers and call a chameleond method",
		Contacts: []string{
			"jaredbennett@google.com",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithBluetoothEnabled",
		Vars:         bluetooth.TastVars,
	})
}

// TestConnectToBTPeers tests that a test can connect to btpeers and call a
// chameleond method.
func TestConnectToBTPeers(ctx context.Context, s *testing.State) {
	btpeers, err := bluetooth.ConnectToBTPeers(ctx, s.RequiredVar(bluetooth.BTPeersVar), 2)
	if err != nil {
		s.Fatal("Failed to connect to 2 btpeers: ", err)
	}
	if _, err := btpeers[0].GetMacAddress(ctx); err != nil {
		s.Fatal("Failed to call chamleleond method 'GetMacAddress' on btpeer1: ", err)
	}
	if success, err := btpeers[1].GetBluetoothAudioDevice().Reboot(ctx); err != nil {
		s.Fatal("Failed to call chamleleond method 'Reboot' on btpeer2.BluetoothAudioDevice: ", err)
	} else if !success {
		s.Fatal("Call to chamleleond method 'Reboot' on btpeer2.BluetoothAudioDevice was processed but was unsuccessful")
	}
}
