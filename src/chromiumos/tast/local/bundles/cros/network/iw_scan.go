// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/network/iw"
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
		iface = "wlan0"
		props = "wifi"
	)
	// In order to guarantee reliable execution of IWScan, we need to make sure
	// shill doesn't interfere with the scan. We will disable shill's control
	// on the wireless device while still maintaining ethernet connectivity.
	manager, err := shill.NewManager(ctx)
	defer func() {
		// Allow shill to take control of wireless device.
		s.Log("Enabling Wifi with shill")
		if err := manager.EnableTechnology(ctx, props); err != nil {
			s.Fatal("Could not enable wifi from shill: ", err)
		}
		s.Log("Wifi Enabled")
	}()
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// Make sure shill doesn't interfere with scans on suspend/resume.
	s.Log("Disabling wifi from shill")
	if err := manager.DisableTechnology(ctx, props); err != nil {
		s.Fatal("Could not disable wifi from shill: ", err)
	}
	s.Log("Wifi disabled")

	// Bring up wireless device after it's released from shill.
	s.Log("Enable wifi manually")
	if err := testexec.CommandContext(ctx, "ip", "link", "set", iface, "up").Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Could not bring up %s after disable", iface)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := iw.TimedScan(ctx, iface, nil, nil)
		if err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
		s.Fatal("TimedScan failed: ", err)
	}

}
