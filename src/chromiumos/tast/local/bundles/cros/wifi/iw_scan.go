// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/local/network/ip"
	"chromiumos/tast/local/network/iw"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: IWScan,
		Desc: "Verifies `iw` Timed Scan executes and is parsed properly",
		Contacts: []string{
			"deanliao@google.com",             // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
		HardwareDeps: hwdep.D(hwdep.WifiNotMarvell()),
	})
}

func IWScan(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	iface, err := shill.WifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		s.Fatal("Could not get a WiFi interface: ", err)
	}
	s.Log("WiFi interface: ", iface)

	// In order to guarantee reliable execution of IWScan, we need to make sure
	// shill doesn't interfere with the scan. We will disable shill's control
	// on the wireless device while still maintaining Ethernet connectivity.
	if err := manager.DisableTechnology(ctx, shill.TechnologyWifi); err != nil {
		s.Fatal("Could not disable WiFi from shill: ", err)
	}

	defer func() {
		// Allow shill to take control of wireless device.
		if err := manager.EnableTechnology(ctx, shill.TechnologyWifi); err != nil {
			s.Error("Could not enable WiFi from shill: ", err)
		}
	}()

	// Bring up wireless device after it's released from shill.
	ipr := ip.NewLocalRunner()
	if err := ipr.SetLinkUp(ctx, iface); err != nil {
		s.Fatalf("Could not bring up %s after shill released WiFi management", iface)
	}

	// Conduct scan
	iwr := iw.NewLocalRunner()
	if _, err = iwr.TimedScan(ctx, iface, nil, nil); err != nil {
		s.Fatal("TimedScan failed: ", err)
	}
}
