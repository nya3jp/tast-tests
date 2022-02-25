// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/network/netconfig"
	. "chromiumos/tast/local/network/netconfig"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MojoCellularToggleCompare,
		Desc:         "Enable/disable Cellular service using Mojo and check WiFi/Ethernet are not affected",
		Contacts:     []string{"shijinabraham@google.com", "cros-network-health@google.com", "chromeos-cellular-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable"},
		Timeout:      10 * time.Minute,
		Fixture:      "cellular",
	})
}

// compareDeviceStatelist compare the network state before starting the test with the state during the test
func compareDeviceStatelist(ctx context.Context, s *testing.State, orgDeviceState []DeviceStateProperties, curDeviceState []DeviceStateProperties, isEnabled bool) {
	for _, cfg := range curDeviceState {
		deviceType := cfg.Type
		deviceState := cfg.DeviceState
		// check cellular device against the known status
		if deviceType == Cellular {

			if isEnabled {
				if deviceState != EnabledDST {
					s.Fatal("Cellular should be enabled but deviceState is %d", deviceState)
				}
			} else {
				if deviceState != DisabledDST {
					s.Fatal("Cellular should be disable but deviceState is %d", deviceState)
				}
			}
		}

		// Checking only Ethernet and WiFi
		if deviceType != Ethernet && deviceType != WiFi {
			continue
		}

		// Ethernet and WiFi should maintain the original state
		for _, cfg := range orgDeviceState {
			if cfg.Type == deviceType && cfg.DeviceState != deviceState {
				s.Fatal("Device %d state expect %d got %d", cfg.Type, cfg.DeviceState, deviceState)
			}
		}
	}
}

// MojoCellularToggle enables/distable cellular network using Mojo and confirms using mojo that WiFi/Ethernet are not affected
func MojoCellularToggleCompare(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed start Chrome: ", err)
	}
	defer cr.Close(ctx)

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	_, err = helper.Enable(ctx)
	if err != nil {
		s.Fatal("Failed to enable cellular: ", err)
	}

	// Enable Wifi
	wifiManager, err := shill.NewWifiManager(ctx, nil)
	if err != nil {
		s.Fatal("Failed to create shill Wi-Fi manager: ", err)
	}
	if err := wifiManager.Enable(ctx, true); err != nil {
		s.Fatal("Failed to enable Wi-Fi: ", err)
	}

	netConn, err := netconfig.NewCrosNetworkConfig(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get network Mojo Object: ", err)
	}

	orgDeviceState, err := netConn.GetDeviceStateList(ctx)
	if err != nil {
		s.Fatal("Failed to get DeviceStateList: ", err)
	}
	s.Logf("DevStateList before test is is %+v", orgDeviceState)

	const iterations = 5
	for i := 0; i < iterations; i++ {
		var isEnabled bool
		if i%2 == 0 {
			isEnabled = false
		} else {
			isEnabled = true
		}

		s.Logf("Toggling Cellular state to %t (iteration %d of %d)", isEnabled, i+1, iterations)

		if err = netConn.SetNetworkTypeEnabledState(ctx, netconfig.Cellular, isEnabled); err != nil {
			s.Fatal("Failed to set cellular state: ", err)
		}

		if err = helper.WaitForEnabledState(ctx, isEnabled); err != nil {
			s.Fatal("cellular state is not as expected: ", err)
		}

		curDeviceState, err := netConn.GetDeviceStateList(ctx)
		if err != nil {
			s.Fatal("Failed to get DeviceStateList: ", err)
		}
		s.Logf("Current DevStatelist is is %+v", curDeviceState)
		time.Sleep(5 * time.Second)
		compareDeviceStatelist(ctx, s, orgDeviceState, curDeviceState, isEnabled)
	}
}
