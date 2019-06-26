// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network contains local Tast tests that exercise the Chrome OS network stack.
package network

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     WifiReset,
		Desc:     "Ensures that WiFi chip can recover from suspend resume properly",
		Contacts: []string{"billyzhao@google.com"},
	})
}
func WifiReset(ctx context.Context, s *testing.State) {
	_, err := iw.TimedScan(ctx, "wlan0", nil, nil)
	if err != nil {
		s.Fatal(errors.Wrap(err, "First Scan failed.").Error())
	}
	s.Log("First Scan succeeded")
	defer func() {
		testexec.CommandContext(ctx, "sh", "-c", "start shill").Output()
		s.Log("starting shill")
		testexec.CommandContext(ctx, "sh", "-c", "start wpasupplicant").Output()
		s.Log("starting wpa_supplicant")
	}()

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
			time.Sleep(1000 * time.Millisecond)
		}
		testexec.CommandContext(ctx, "sh", "-c", "stop shill").Output()
		s.Log("stopping shill")
		testexec.CommandContext(ctx, "sh", "-c", "stop wpasupplicant").Output()
		s.Log("stopping wpa_supplicant")

		testexec.CommandContext(ctx, "sh", "-c", "ip link set wlan0 up").Output()
		s.Log("Bring up wlan0")
		_, err = iw.TimedScan(ctx, "wlan0", nil, nil)
		if err != nil {
			s.Fatal(errors.Wrap(err, "Second Scan failed.").Error())
		}
		s.Log(fmt.Sprintf("Second Scan succeeded"))
		testexec.CommandContext(ctx, "sh", "-c", "start shill").Output()
		s.Log("starting shill")
		testexec.CommandContext(ctx, "sh", "-c", "start wpasupplicant").Output()
		s.Log("starting wpa_supplicant")
	}

}
