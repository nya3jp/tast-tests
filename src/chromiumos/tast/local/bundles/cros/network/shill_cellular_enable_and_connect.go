// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/local/shill"
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
	for i := 0; i < 3; i++ {
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
	// TODO(b:190541087): Use helper.Disable instead.
	// Currently that causes ssh timeouts in the test runner for unknown reasons.
	// This inlines helper.Disable with logging in between.
	s.Log("Disable Cellular while Connected")
	if err := helper.Manager.DisableTechnology(ctx, shill.TechnologyCellular); err != nil {
		s.Fatal("Disable failed: ", err)
	}

	s.Log("Wait for disabled")
	if err := helper.WaitForEnabledState(ctx, false); err != nil {
		s.Fatal("Wait for disable failed: ", err)
	}
	s.Log("Wait for !powered")
	if err := helper.Device.WaitForProperty(ctx, shillconst.DevicePropertyPowered, false, shillconst.DefaultTimeout); err != nil {
		s.Fatal("Wait for !powered failed: ", err)
	}

	s.Log("Ensure no Cellular Service while disabled")
	const shortTimeout = 3 * time.Second
	if _, err := helper.FindServiceForDeviceWithTimeout(ctx, shortTimeout); err == nil {
		s.Fatal("Service found while Disabled")
	}

	s.Log("Final Enable")
	if err := helper.Enable(ctx); err != nil {
		s.Fatal("Final Enable failed: ", err)
	}
}
