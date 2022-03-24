// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/wifi/wlan"
	"chromiumos/tast/local/network/iw"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: IwDumps,
		Desc: "Dump SoC capabilities",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:mainline", "group:wificell", "wificell_func"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
	})
}

func IwDumps(ctx context.Context, s *testing.State) {
	iwr := iw.NewLocalRunner()
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
		s.Fatal("Failed reading the WLAN device information: ", err)
	}

	text, err := iwr.DumpPhys(ctx)
	if err != nil {
		s.Fatal("DumpPhys failed: ", err)
	}
	if len(text) == 0 {
		s.Fatal("Empty iw list result")
	}

	s.Log(text)
}
