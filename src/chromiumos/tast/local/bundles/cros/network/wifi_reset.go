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
	"chromiumos/tast/local/bundles/cros/network/ping"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     WifiReset,
		Desc:     "",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

func resetDriver(ctx context.Context) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := testexec.CommandContext(ctx, "modprobe", "-r", "iwlmvm", "iwlwifi").Run(testexec.DumpLogOnError); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: time.Millisecond * 500}); err != nil {
		return err
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := testexec.CommandContext(ctx, "modprobe", "iwlwifi").Run(testexec.DumpLogOnError); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: time.Millisecond * 500}); err != nil {
		return err
	}
	return nil
}
func integrateShillWifi(ctx context.Context, s *testing.State, manager *shill.Manager) {
	// Allow shill to take control of wireless device.
	s.Log("Enabling Wifi with shill")
	if err := manager.EnableTechnology(ctx, "wifi"); err != nil {
		s.Fatal("Could not enable wifi from shill: ", err)
	}
	s.Log("Wifi Enabled")
}

func isolateShillWifi(ctx context.Context, s *testing.State, manager *shill.Manager) {
	// Make sure shill doesn't interfere with scans on suspend/resume.
	s.Log("Disabling wifi from shill")
	if err := manager.DisableTechnology(ctx, "wifi"); err != nil {
		s.Fatal("Could not disable wifi from shill: ", err)
	}
	s.Log("Wifi disabled")

	// Bring up wireless device after it's released from shill.
	s.Log("Enable wifi manually")
	if err := testexec.CommandContext(ctx, "ifconfig", "wlan0", "up").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Could not bring up wlan0 after disable")
	}
}

func getIface(ctx context.Context, s *testing.State) string {
	ifaces, err := iw.ListInterfaces(ctx)
	if err != nil {
		s.Fatal("ListInterfaces failed: ")
	}
	if len(ifaces) != 1 {
		s.Fatal("Unexpected number of wireless interfaces")
	}
	return ifaces[0].IfName
}

func WifiReset(ctx context.Context, s *testing.State) {
	const (
		numResets   int = 15
		numSuspends int = 5
	)
	props := map[string]interface{}{
		"Type":          "wifi",
		"Name":          "GoogleGuest",
		"SecurityClass": "none",
		"Mode":          "managed",
	}

	iface := getIface(ctx, s)
	// Hook into shill service.
	manager, err := shill.NewManager(ctx)
	defer integrateShillWifi(ctx, s, manager)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	for i := 0; i < numSuspends; i++ {
		s.Logf("Running suspend # %d", i)

		isolateShillWifi(ctx, s, manager)
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
		integrateShillWifi(ctx, s, manager)
		for j := 0; j < numResets; j++ {
			s.Logf("Running Reset # %d", j)
			if err := resetDriver(ctx); err != nil {
				s.Fatal("Could not reset WifiDriver.", err)
			}
			manager.DisconnectFromWifiNetwork(ctx, props)
			err := manager.ConnectToWifiNetwork(ctx, props)
			if err == nil {
				if _, err := ping.SimplePing(ctx, "8.8.8.8"); err != nil {
					s.Fatal("Could not ping successfully", err)
				}

			}
		}

	}
}
