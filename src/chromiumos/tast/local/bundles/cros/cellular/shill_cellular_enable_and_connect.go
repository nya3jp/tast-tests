// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularEnableAndConnect,
		Desc:     "Verifies that Shill can enable, disable, connect, and disconnect to a Cellular Service",
		Contacts: []string{"stevenjb@google.com", "cros-network-health@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
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

	perfValues := perf.NewValues()

	// Test Disable / Enable / Connect / Disconnect.
	// Run the test a second time to test Disable after Connect/Disconnect.
	// Run the test a third time to help test against flakiness.
	for i := 0; i < 3; i++ {
		s.Logf("Disable %d", i)
		disableTime, err := helper.Disable(ctx)
		if err != nil {
			s.Fatalf("Disable failed on attempt %d: %s", i, err)
		}
		perfValues.Append(perf.Metric{
			Name:      "cellular_disable_time",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, disableTime.Seconds())
		s.Logf("Enable %d", i)
		enableTime, err := helper.Enable(ctx)
		if err != nil {
			s.Fatalf("Enable failed on attempt %d: %s", i, err)
		}
		perfValues.Append(perf.Metric{
			Name:      "cellular_enable_time",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, enableTime.Seconds())
		s.Logf("Connect %d", i)
		connectTime, err := helper.ConnectToDefault(ctx)
		if err != nil {
			s.Fatalf("Connect failed on attempt %d: %s", i, err)
		}
		perfValues.Append(perf.Metric{
			Name:      "cellular_connect_time",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, connectTime.Seconds())
		s.Logf("Disconnect %d", i)
		disconnectTime, err := helper.Disconnect(ctx)
		if err != nil {
			s.Fatalf("Disconnect failed on attempt %d: %s", i, err)
		}
		perfValues.Append(perf.Metric{
			Name:      "cellular_disconnect_time",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, disconnectTime.Seconds())
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}

	// Test that Disconnect fails while not connected.
	if _, err := helper.Disconnect(ctx); err == nil {
		s.Fatal("Disconnect succeeded while disconnected: ", err)
	}

	s.Log("Reconnect")
	if _, err := helper.ConnectToDefault(ctx); err != nil {
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
	if _, err := helper.Enable(ctx); err != nil {
		s.Fatal("Final Enable failed: ", err)
	}
}
