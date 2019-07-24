// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network contains local Tast tests that exercise the Chrome OS network stack.
package network

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/iw"
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
	const (
		iface = "wlan0"
		props = "wifi"
	)
	// Hook into shill service.
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
	if err := testexec.CommandContext(ctx, "ifconfig", "wlan0", "up").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Could not bring up wlan0 after disable")
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
			o, err := testexec.CommandContext(ctx, "ip", "add", "sh", "dev", iface).Output()
			if err != nil {
				return err
			} else if !strings.Contains(string(o), "wlan") {
				return errors.New("could not find wlan device")
			} else {
				return nil
			}
		}, &testing.PollOptions{Timeout: time.Second}); err != nil {
			s.Fatal("Device failed to be brought up: ", err)
		}

		// Scan
		if _, err := iw.TimedScan(ctx, "wlan0", nil, nil); err != nil {
			s.Fatal(errors.Wrap(err, "scan failed").Error())
		}
		s.Log(fmt.Sprintf("Scan succeeded"))
	}
}
