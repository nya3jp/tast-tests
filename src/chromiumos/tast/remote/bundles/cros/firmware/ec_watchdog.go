// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECWatchdog,
		Desc:         "Servo based EC watchdog test",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"}
		Attr:         []string{"group:firmware", "firmware_smoke"},
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func ECWatchdog(ctx context.Context, s *testing.State) {

	const (
		WATCHDOG_DELAY = 3700
		EC_BOOT_DELAY  = 1000
	)

	d := s.DUT()

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	s.Log("Trigger a watchdog reset and power on system again")
	err = d.Command("sync")
	if err != nil {
		s.Fatal("Failed to sync IO on DUT before calling watchdog: ", err)
	}
	s.Log("Trigger watchdog event")
	err = pxy.Servo().RunECCommand(ctx, fmt.Sprintf("waitms %d", WATCHDOG_DELAY))
	if err != nil {
		s.Fatal("Failed to send watchdog timer command to EC: ", err)
	}
	s.Log("Sleep during watchdog reset")
	if err = testing.Sleep(ctx, WATCHDOG_DELAY+EC_BOOT_DELAY/1000); err != nil {
		s.Fatal("Failed to sleep during waiting for EC to get up: ", err)
	}
	s.Log("Wait for DUT to reconnect")
	if err = d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}
}
