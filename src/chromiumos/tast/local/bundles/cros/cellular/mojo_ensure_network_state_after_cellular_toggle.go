// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/network/netconfig"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MojoEnsureNetworkStateAfterCellularToggle,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Enable/disable Cellular service using Mojo and check WiFi/Ethernet are not affected",
		Contacts:     []string{"shijinabraham@google.com", "cros-network-health@google.com", "chromeos-cellular-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable"},
		Timeout:      10 * time.Minute,
		Fixture:      "cellular",
	})
}

// compareDeviceStatelist compares the device states with states before toggling cellular device.
// cellular device state should match 'enabled' argument and Ethernet and WiFi should maintain the original state
func compareDeviceStatelist(orgDeviceState, curDeviceState []netconfig.DeviceStateProperties, enabled bool) error {
	for _, cfg := range curDeviceState {
		deviceType := cfg.Type
		deviceState := cfg.DeviceState
		// Check cellular device against the known status.
		if deviceType == netconfig.Cellular {
			if enabled && deviceState != netconfig.EnabledDST {
				return errors.Errorf("unexpected Cellular state expected: enabled got: %d", deviceState)

			} else if !enabled && deviceState != netconfig.DisabledDST {
				return errors.Errorf("unexpected Cellular state expected: disabled got: %d", deviceState)
			}
		}

		// Checking only Ethernet and WiFi.
		if deviceType != netconfig.Ethernet && deviceType != netconfig.WiFi {
			continue
		}

		// Ethernet and WiFi should maintain the original state.
		for _, cfg := range orgDeviceState {
			if cfg.Type == deviceType && cfg.DeviceState != deviceState {
				return errors.Errorf("unexpected state for device %d, expected: %d, got: %d", cfg.Type, cfg.DeviceState, deviceState)
			}
		}
	}
	return nil
}

// MojoEnsureNetworkStateAfterCellularToggle enables/disables cellular device using Mojo and confirms using mojo that WiFi/Ethernet are not affected.
func MojoEnsureNetworkStateAfterCellularToggle(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed start Chrome: ", err)
	}
	defer cr.Close(ctx)

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Enable Wifi.
	wifiManager, err := shill.NewWifiManager(ctx, nil)
	if err != nil {
		s.Fatal("Failed to create shill Wi-Fi manager: ", err)
	}
	if err := wifiManager.Enable(ctx, true); err != nil {
		s.Fatal("Failed to enable Wi-Fi: ", err)
	}

	netConn, err := netconfig.CreateLoggedInCrosNetworkConfig(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get network Mojo Object: ", err)
	}
	defer netConn.Close(ctx)

	orgDeviceState, err := netConn.GetDeviceStateList(ctx)
	if err != nil {
		s.Fatal("Failed to get DeviceStateList: ", err)
	}

	const iterations = 5
	for i := 0; i < iterations; i++ {
		enabled := i%2 != 0
		s.Logf("Toggling Cellular state to %t (iteration %d of %d)", enabled, i+1, iterations)

		if err = netConn.SetNetworkTypeEnabledState(ctx, netconfig.Cellular, enabled); err != nil {
			s.Fatal("Failed to set cellular state: ", err)
		}

		if err = helper.WaitForEnabledState(ctx, enabled); err != nil {
			s.Fatal("cellular state is not as expected: ", err)
		}

		curDeviceState, err := netConn.GetDeviceStateList(ctx)
		if err != nil {
			s.Fatal("Failed to get DeviceStateList: ", err)
		}
		if err := compareDeviceStatelist(orgDeviceState, curDeviceState, enabled); err != nil {
			s.Fatal("Device state comparison failed: ", err)
		}
	}
}
