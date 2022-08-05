// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"chromiumos/tast/remote/bluetooth"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestConnectToBTPeers,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that a remote test can connect to btpeers and call a chameleond method",
		Contacts: []string{
			"jaredbennett@google.com",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWith2BTPeers",
		Timeout:      time.Second * 15,
	})
}

// TestConnectToBTPeers tests that a remote test can connect to btpeers and call
// a chameleond method.
func TestConnectToBTPeers(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*bluetooth.FixtValue)

	if _, err := fv.BTPeers[0].GetMacAddress(ctx); err != nil {
		s.Fatal("Failed to call chamleleond method 'GetMacAddress' on btpeer1: ", err)
	}
	if err := fv.BTPeers[1].BluetoothAudioDevice().Reboot(ctx); err != nil {
		s.Fatal("Failed to call chamleleond method 'Reboot' on btpeer2.BluetoothAudioDevice: ", err)
	}
}
