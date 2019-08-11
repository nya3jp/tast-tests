// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
// Package network contains local Tast tests that exercise the Chrome OS network stack.
package network

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/local/bundles/cros/network/ping"
	"chromiumos/tast/local/network"
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

func isolateShillWifi(ctx context.Context, s *testing.State, manager *shill.Manager, iface string) {
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
}

var aps = []map[string]interface{}{
	{
		"Type":          "wifi",
		"Name":          "jellytime",
		"SecurityClass": "none",
		"Mode":          "managed",
	},
	{
		"Type":          "wifi",
		"Name":          "peanutbutter",
		"SecurityClass": "none",
		"Mode":          "managed",
	},
	{
		"Type":          "wifi",
		"Name":          "GoogleGuest",
		"SecurityClass": "none",
		"Mode":          "managed",
	},
}

func selectAP(ctx context.Context, manager *shill.Manager) (map[string]interface{}, int, error) {
	var str uint8 = 0
	var bestAP map[string]interface{}
	index := -1
	for i, ap := range aps {
		path, err := manager.FindMatchingService(ctx, ap)
		if err != nil {
			continue
		}
		ap, err = shill.GetPropsForService(ctx, path)
		if err != nil {
			continue
		}
		s, ok := ap["Stength"].(uint8)
		if ok {
			return nil, -1, errors.Wrapf(err, "parsing error on strength field %T", ap["Strength"])
		}
		if s >= str {
			str = s
			index = i
			bestAP = ap
		}
	}
	if index == -1 {
		return nil, -1, errors.New("could not find matching service")
	}
	return bestAP, index, nil
}
func WifiReset(ctx context.Context, s *testing.State) {
	const (
		numResets   int = 5
		numSuspends int = 2
	)
	iface, err := network.FindWifiInterface(ctx)
	if err != nil {
		s.Fatal("Failed to find wireless interface: ", err)
	}

	// Hook into shill service.
	manager, err := shill.NewManager(ctx)
	defer integrateShillWifi(ctx, s, manager)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	bestAP, index, err := selectAP(ctx, manager)
	props := aps[index]
	if err != nil {
		s.Fatal(err)
	}
	s.Logf("Wifi SSID: %s - Wifi Strength %d", bestAP["Name"], bestAP["Strength"])

	for i := 0; i < numSuspends; i++ {
		s.Logf("Running suspend # %d", i)

		isolateShillWifi(ctx, s, manager, iface)
		if err := testexec.CommandContext(ctx, "suspend_stress_test", "-c", "1").Run(testexec.DumpLogOnError); err != nil {
			s.Fatal(errors.Wrapf(err, "Suspend/resume %d failed", i))
		}
		s.Logf("Suspend/resume %d succeeded", i)

		// Check wireless device still available
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			interf, err := network.FindWifiInterface(ctx)
			if interf != iface {
				return errors.New("different wireless device found")
			}
			return err
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatal("Device failed to be brought up: ", err)
		}
		// Scan
		if _, err := iw.TimedScan(ctx, iface, nil, nil); err != nil {
			s.Fatal(errors.Wrap(err, "scan failed").Error())
		}
		s.Log(fmt.Sprintf("Scan succeeded"))
		integrateShillWifi(ctx, s, manager)
		for j := 0; j < numResets; j++ {
			s.Logf("Running Reset # %d", j)

			s.Logf("%v", props)
			manager.DisconnectFromWifiNetwork(ctx, props)
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if res, err := ping.SimplePing(ctx, "8.8.8.8"); err != nil || !res {
					return nil
				}
				return errors.New("ping should fail.")

			}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: time.Millisecond * 100}); err != nil {
				s.Fatalf("Could not disconnect from wifi", err)
			}
			err := manager.ConnectToWifiNetwork(ctx, props)
			if err != nil {
				s.Fatalf("Could not connect to wifi: ", err)
			}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				s.Log("Pinging")
				if _, err := ping.SimplePing(ctx, "8.8.8.8"); err != nil {
					return err
				}
				return nil

			}, &testing.PollOptions{Timeout: 20 * time.Second, Interval: time.Second}); err != nil {
				s.Fatal("Could not ping in time ", err)
			}
		}

	}
}
