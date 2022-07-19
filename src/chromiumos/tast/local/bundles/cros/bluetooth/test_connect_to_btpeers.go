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
	if err := btpeers[0].ResetBluetoothRef(ctx); err != nil {
		s.Fatal("Failed to call chamleleond method 'ResetBluetoothRef' on btpeer1: ", err)
	}
}
