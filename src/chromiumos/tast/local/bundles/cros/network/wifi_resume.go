// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network contains local Tast tests that exercise the Chrome OS network stack.
package network

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     WifiResume,
		Desc:     "Ensures that WiFi chip can recover from suspend resume properly",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}
func WifiResume(ctx context.Context, s *testing.State) {
	iface, err := network.FindWifiInterface(ctx)
	if err != nil {
		s.Fatal("Could not get wireless interface: ", err)
	}
	// Hook into shill service.
	manager, err := shill.NewManager(ctx)
	defer func() {
		// Allow shill to take control of wireless device.
		s.Log("Enabling Wifi with shill")
		if err := manager.EnableTechnology(ctx, "wifi"); err != nil {
			s.Fatal("Could not enable wifi from shill: ", err)
		}
		s.Log("Wifi Enabled")
	}()
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// Make sure shill doesn't interfere with scans on suspend/resume.
	s.Log("Disabling wifi from shill")
	if err := manager.DisableTechnology(ctx, "wifi"); err != nil {
		s.Fatal("Could not disable wifi from shill: ", err)
	}
	s.Log("Wifi disabled")

	// Bring up wireless device after it's released from shill.
	s.Log("Enable wifi manually")
	if err := testexec.CommandContext(ctx, "ip", "link", "set", iface, "up").Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Could not bring up %s after disable", iface)
	}

	// Loop suspend/resume
	for i := 0; i < 5; i++ {
		s.Logf("Attempt suspend/resume %d", i)
		if err := testexec.CommandContext(ctx, "suspend_stress_test", "-c", "1").Run(testexec.DumpLogOnError); err != nil {
			s.Fatal(errors.Wrapf(err, "Suspend/resume %d failed", i))
		}
		s.Logf("Suspend/resume %d succeeded", i)

		// Check wireless device still available
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			_, err := network.FindWifiInterface(ctx)
			if err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Second}); err != nil {
			s.Fatal("Device failed to be brought up: ", err)
		}

		// Scan
		if _, err := iw.TimedScan(ctx, iface, nil, nil); err != nil {
			s.Fatal(errors.Wrap(err, "scan failed").Error())
		}
		s.Log("Scan succeeded")
	}
}
