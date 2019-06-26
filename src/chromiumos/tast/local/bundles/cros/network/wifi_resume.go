// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network contains local Tast tests that exercise the Chrome OS network stack.
package network

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/iw"
	//"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"context"
	"fmt"
	//"os"
	"strings"
	//"time"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     WifiResume,
		Desc:     "Ensures that WiFi chip can recover from suspend resume properly",
		Contacts: []string{"billyzhao@google.com"},
	})
}
func WifiResume(ctx context.Context, s *testing.State) {
	flag := false
	manager, err := shill.NewManager(ctx)
	var props interface{} = "wifi"
	defer func() {
		if !flag {
			s.Log("Enabling Wifi")
			if err := manager.EnableTechnology(ctx, props); err != nil {
				s.Fatal("Could not enable wifi from shill: ", err)
			}
			s.Log("Wifi Enabled")

		}
	}()

	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	s.Log("Disabling wifi")
	if err := manager.DisableTechnology(ctx, props); err != nil {
		s.Fatal("Could not disable wifi from shill: ", err)
	}
	if err := testexec.CommandContext(ctx, "ifconfig", "wlan0", "up").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Could not bring up wlan0 after disable")
	}
	s.Log("Wifi disabled")
	for i := 0; i < 5; i++ {
		_, err = testexec.CommandContext(ctx, "suspend_stress_test", "-c", "1").Output()
		if err != nil {
			s.Fatal(errors.Wrap(err, "Reset failed.").Error())
		}
		s.Log("Suspend resume succeeded.")
		for flag := false; flag == false; {
			o, err := testexec.CommandContext(ctx, "ip", "link", "show").Output()
			if err != nil {
				s.Fatal(errors.Wrap(err, "ip link failed").Error())
			}
			if strings.Contains(string(o), "wlan") {
				flag = true
			}
			s.Log(string(o))
		}

		_, err = iw.TimedScan(ctx, "wlan0", nil, nil)
		if err != nil {
			s.Fatal(errors.Wrap(err, "Scan failed.").Error())
		}
		s.Log(fmt.Sprintf("Scan succeeded"))
		//start
	}
	s.Log("Enabling Wifi")
	if err := manager.EnableTechnology(ctx, props); err != nil {
		s.Fatal("Could not enable wifi from shill: ", err)
	}
	s.Log("Wifi Enabled")
	flag = true
}
