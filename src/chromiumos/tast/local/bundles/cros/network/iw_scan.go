// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     IWScan,
		Desc:     "Verifies `iw` Timed Scan executes and is parsed properly",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

func IWScan(ctx context.Context, s *testing.State) {
	const (
		technology = "wifi"
	)
	iface, err := network.FindWirelessInterface(ctx)
	if err != nil {
		s.Fatal("Could not find wireless interface: ", err)
	}
	// In order to guarantee reliable execution of IWScan, we need to make sure
	// shill doesn't interfere with the scan. We will disable shill's control
	// on the wireless device while still maintaining ethernet connectivity.
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// Make sure shill doesn't interfere with scans on suspend/resume.
	if err := manager.DisableTechnology(ctx, technology); err != nil {
		s.Fatal("Could not disable wifi from shill: ", err)
	}

	defer func() {
		// Allow shill to take control of wireless device.
		if err := manager.EnableTechnology(ctx, technology); err != nil {
			s.Fatal("Could not enable wifi from shill: ", err)
		}
	}()

	// Bring up wireless device after it's released from shill.
	if err := testexec.CommandContext(ctx, "ip", "link", "set", iface, "up").Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Could not bring up %s after disable", iface)
	}

	// Conduct scan
	if _, err = iw.TimedScan(ctx, iface, nil, nil); err != nil {
		s.Fatal("TimedScan failed: ", err)
	}
}
