// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/network/wlan"
	"chromiumos/tast/local/network/iw"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WifiCaps,
		Desc:         "Verifies DUT supports a minimum set of required protocols",
		Contacts:     []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
		Params: []testing.Param{
			{
				// Verifies basic capabilities.
				Name: "",
				Val:  basicCheck,
			},
			{
				// Verifies 802.11ac capabilities.
				Name:              "80211ac",
				Val:               acCaps,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			},
		},
	})
}

// basicCheck is the test body of checking basic WiFi capabilities.
func basicCheck(ctx context.Context, s *testing.State) {
	iwr := iw.NewRunner()
	// Get WiFi interface.
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// GetWifiInterface returns the wireless device interface name (e.g. wlan0), or returns an error on failure.
	netIf, err := shill.WifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		s.Fatal("Could not get a WiFi interface: ", err)
	}

	// Get the information of the WLAN device.
	dev, err := wlan.DeviceInfo(ctx, netIf)
	if err != nil {
		s.Fatal(err, "Failed reading the WLAN device information")
	}

	res, err := iwr.ListPhys(ctx)
	if err != nil {
		s.Fatal("ListPhys failed: ", err)
	}
	if len(res) == 0 {
		s.Fatal("ListPhys returned no device")
	}

	staSupported := false
	for _, mode := range res[0].Modes {
		if mode == "managed" {
			staSupported = true
		}
	}
	if !staSupported {
		s.Error("Station mode not supported")
	}

	// Check both 2.4 GHz and 5 GHz bands supported.
	supported24 := false
	supported5 := false
	for _, band := range res[0].Bands {
		for freq := range band.FrequencyFlags {
			if freq >= 2400 && freq <= 2499 {
				supported24 = true
			} else if freq >= 5000 && freq <= 5999 {
				supported5 = true
			}
		}
	}
	if !supported24 {
		s.Error("Device doesn't support 2.4ghz")
	}
	if !supported5 {
		s.Error("Device doesn't support 5ghz bands")
	}
	if !res[0].SupportHT2040 {
		s.Error("Device doesn't support all required throughput options: HT20, HT40")
	}
	// Check short guard interval support.
	if !res[0].SupportHT20SGI {
		s.Error("Device doesn't support HT20 short guard interval")
	}
	if !res[0].SupportHT40SGI {
		s.Error("Device doesn't support HT40 short guard interval")
	}

	// Check MU-MIMO support. Older generations don't support MU-MIMO.
	// TODO(crbug.com/1024554): Move to acCaps after it is critical or merge
	// the two after monroe EOL.
	if dev.SupportMUMIMO() != res[0].SupportMUMIMO {
		if dev.SupportMUMIMO() {
			// New chips require MU-MIMO.
			s.Error("Device doesn't support MU-MIMO")
		} else {
			// We do not expect older chips to have MU-MIMO support.
			s.Error("Device unexpectedly supports MU-MIMO")
		}
	}
}

func acCaps(ctx context.Context, s *testing.State) {
	iwr := iw.NewRunner()
	res, err := iwr.ListPhys(ctx)
	if err != nil {
		s.Fatal("ListPhys failed: ", err)
	}
	if !res[0].SupportVHT {
		s.Error("Device doesn't support VHT")
	}
	if !res[0].SupportVHT80SGI {
		s.Error("Device doesn't support VHT80 short guard interval")
	}
}

func WifiCaps(ctx context.Context, s *testing.State) {
	f := s.Param().(func(context.Context, *testing.State))
	f(ctx, s)
}
