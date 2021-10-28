// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

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

	s.Log("Send 'chan 0' to EC")
	if err := h.Servo.RunECCommand(ctx, "chan 0"); err != nil {
		s.Fatal("Failed to send 'chan 0' to EC: ", err)
	}
	defer func() {
		s.Log("Send 'chan 0xffffffff' to EC")
		if err := h.Servo.RunECCommand(ctx, "chan 0xffffffff"); err != nil {
			s.Fatal("Failed to send 'chan 0xffffffff' to EC: ", err)
		}
	}()

	s.Log("Poweroff DUT")
	if err := ms.PowerOff(ctx); err != nil {
		s.Fatal("Failed to poweroff DUT: ", err)
	}
	defer h.WaitConnect(ctx)

	s.Log("Check for G3 powerstate")
	if err := ms.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3"); err != nil {
		s.Fatal("Failed to get G3 powerstate: ", err)
	}

	s.Log("Power DUT back on")
	if err := h.Servo.PowerShortPress(ctx); err != nil {
		s.Fatal("Failed to power on DUT with short press of the power button: ", err)
	}
}
