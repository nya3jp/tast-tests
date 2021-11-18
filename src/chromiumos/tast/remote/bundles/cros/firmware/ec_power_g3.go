// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECPowerG3,
		Desc:         "Test that DUT goes to G3 powerstate on shutdown",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func ECPowerG3(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create mode switcher: ", err)
	}

	s.Log("Shut down DUT")
	cmd := h.DUT.Conn().CommandContext(ctx, "/sbin/shutdown", "-P", "now")
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to shut down DUT: ", err)
	}

	s.Log("Check for G3 powerstate")
	if err := ms.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3"); err != nil {
		s.Fatal("Failed to get G3 powerstate: ", err)
	}

	s.Log("Power DUT back on with short press of the power button")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
		s.Fatal("Failed to power on DUT with short press of the power button: ", err)
	}

	if err := h.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT after restarting: ", err)
	}
}
