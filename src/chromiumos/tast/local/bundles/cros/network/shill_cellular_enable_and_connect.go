// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularEnableAndConnect,
		Desc:     "Verifies that Shill can enable, disable, connect, and disconnect to a Cellular Service",
		Contacts: []string{"stevenjb@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable"},
		Timeout:  10 * time.Minute,
		Fixture:  "cellular",
	})
}

func ShillCellularEnableAndConnect(ctx context.Context, s *testing.State) {
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Disable AutoConnect so that enable does not connect.
	ctxForAutoConnectCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cellular.AutoConnectCleanupTime)
	defer cancel()
	if wasAutoConnect, err := helper.SetServiceAutoConnect(ctx, false); err != nil {
		s.Fatal("Failed to disable AutoConnect: ", err)
	} else if wasAutoConnect {
		defer func(ctx context.Context) {
			if _, err := helper.SetServiceAutoConnect(ctx, true); err != nil {
				s.Fatal("Failed to enable AutoConnect: ", err)
			}
		}(ctxForAutoConnectCleanUp)
	}

	// Test Disable / Enable / Connect / Disconnect.
	// Run the test a second time to test Disable after Connect/Disconnect.
	// TODO(b:190541087): Run a third time to help test against flakiness.
	for i := 0; i < 2; i++ {
		s.Logf("Disable %d", i)
		if err := helper.Disable(ctx); err != nil {
			s.Fatalf("Disable failed on attempt %d: %s", i, err)
		}
		s.Logf("Enable %d", i)
		if err := helper.Enable(ctx); err != nil {
			s.Fatalf("Enable failed on attempt %d: %s", i, err)
		}
		s.Logf("Connect %d", i)
		if err := helper.ConnectToDefault(ctx); err != nil {
			s.Fatalf("Connect failed on attempt %d: %s", i, err)
		}
		s.Logf("Disconnect %d", i)
		if err := helper.Disconnect(ctx); err != nil {
			s.Fatalf("Disconnect failed on attempt %d: %s", i, err)
		}
	}

	// Test that Disconnect fails while not connected.
	if err := helper.Disconnect(ctx); err == nil {
		s.Fatal("Disconnect succeeded while disconnected: ", err)
	}

	s.Log("Reconnect")
	if err := helper.ConnectToDefault(ctx); err != nil {
		s.Fatal("Reconnect failed: ", err)
	}

	// Test Disable while connected.
	s.Log("Disable")
	if err := helper.Disable(ctx); err != nil {
		s.Fatal("Disable failed while connected: ", err)
	}

	s.Log("Connect while disabled")
	if err := helper.ConnectToDefault(ctx); err == nil {
		s.Fatal("Connect succeeded while Disabled: ", err)
	}

	s.Log("Disconnect while disabled")
	if err := helper.Disconnect(ctx); err == nil {
		s.Fatal("Disconnect succeeded while Disabled: ", err)
	}

	s.Log("Final Enable")
	if err := helper.Enable(ctx); err != nil {
		s.Fatal("Final Enable failed: ", err)
	}
}
