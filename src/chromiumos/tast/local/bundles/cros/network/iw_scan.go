// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/network/ip"
	"chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     IWScan,
		Desc:     "Verifies `iw` Timed Scan executes and is parsed properly",
		Contacts: []string{"deanliao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func IWScan(ctx context.Context, s *testing.State) {
	if err := shill.SafeStop(ctx); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	} else if err = shill.SafeStart(ctx); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}

	const (
		pollTimeout = 5 * time.Second
	)
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	iface, err := shill.GetWifiInterface(ctx, manager, pollTimeout)
	if err != nil {
		s.Fatal("Could not get a WiFi interface: ", err)
	}
	s.Log("WiFi interface: ", iface)

	// In order to guarantee reliable execution of IWScan, we need to make sure
	// shill doesn't interfere with the scan. We will disable shill's control
	// on the wireless device while still maintaining Ethernet connectivity.
	s.Log("Disable wifi from shill")
	if err := manager.DisableTechnology(ctx, shill.TechnologyWifi); err != nil {
		s.Fatal("Could not disable WiFi from shill: ", err)
	}
	defer func() {
		// Allow shill to take control of wireless device.
		if err := manager.EnableTechnology(ctx, shill.TechnologyWifi); err != nil {
			s.Error("Could not enable WiFi from shill: ", err)
		}
	}()

	ip.PollIfaceUpDown(ctx, iface, false, pollTimeout)

	// Bring up wireless device after it's released from shill.
	s.Logf("Bringing up interface %s", iface)
	if err := ip.SetIfaceUpDown(ctx, iface, true, pollTimeout); err != nil {
		s.Fatalf("Could not bring up %s after shill released WiFi management: %s",
			iface, err.Error())
	}
	ip.PollIfaceUpDown(ctx, iface, true, pollTimeout)

	// Conduct scan
	s.Logf("Running \"iw dev %s scan\"", iface)
	scanData, err := iw.TimedScan(ctx, iface, nil, nil)
	if err != nil {
		s.Fatal("TimedScan failed: ", err)
	}
	s.Logf("Scan time: %s", scanData.Time)
}
