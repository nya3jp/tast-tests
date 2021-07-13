// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECWatchdog,
		Desc:         "Servo based EC watchdog test",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Pre:          pre.NormalMode(),
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func ECWatchdog(ctx context.Context, s *testing.State) {
	const (
		// Delay of spin-wait in ms. Nuvoton boards set the hardware watchdog to
		// 3187.5ms and also sets a timer to 2200ms. Set the timeout long enough to
		// exceed the hardware watchdog timer because the timer isn't 100% reliable.
		// If there are other platforms that use a longer watchdog timeout, this
		// may need to be adjusted.
		WatchdogDelay = 3700
		// Delay of EC power on.
		ECBootDelay = 1000
	)

	h := s.PreValue().(*pre.Value).Helper

	s.Log("Trigger a watchdog reset and power on system again")
	err := h.DUT.Conn().Command("sync")
	if err != nil {
		s.Fatal("Failed to sync IO on DUT before calling watchdog: ", err)
	}
	s.Log("Trigger watchdog event")
	err = h.Servo.RunECCommand(ctx, fmt.Sprintf("waitms %d", WatchdogDelay))
	if err != nil {
		s.Fatal("Failed to send watchdog timer command to EC: ", err)
	}
	s.Log("Sleep during watchdog reset")
	if err = testing.Sleep(ctx, (WatchdogDelay+ECBootDelay)*time.Millisecond); err != nil {
		s.Fatal("Failed to sleep during waiting for EC to get up: ", err)
	}
	s.Log("Wait for DUT to reconnect")
	if err = h.DUT.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}
}
