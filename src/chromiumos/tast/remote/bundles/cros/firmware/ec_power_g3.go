// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type powerOffMethod int

const (
	shutdownCommand powerOffMethod = iota
	longPowerButtonPress
	powerStateOff
)

type powerG3Params struct {
	PowerOffMethod powerOffMethod
	RemovePower    bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECPowerG3,
		Desc:         "Test that DUT goes to G3 powerstate on various types of shutdown",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Params: []testing.Param{
			{
				Name:      "shutdown",
				ExtraAttr: []string{"firmware_ec"},
				Val: powerG3Params{
					PowerOffMethod: shutdownCommand,
				},
			},
			{
				Name:      "power_button",
				ExtraAttr: []string{"firmware_ec", "firmware_bringup"},
				Val: powerG3Params{
					PowerOffMethod: longPowerButtonPress,
				},
			},
			{
				Name:      "power_state",
				ExtraAttr: []string{"firmware_unstable", "firmware_bringup"},
				Val: powerG3Params{
					PowerOffMethod: powerStateOff,
				},
			},
			{
				Name:              "power_state_snk",
				ExtraAttr:         []string{"firmware_unstable", "firmware_bringup"},
				ExtraHardwareDeps: hwdep.D(hwdep.Battery()),
				Val: powerG3Params{
					PowerOffMethod: powerStateOff,
					RemovePower:    true,
				},
			},
		},
	})
}

func ECPowerG3(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	tc := s.Param().(powerG3Params)

	if tc.RemovePower {
		h.SetDUTPower(ctx, false)
	}

	switch tc.PowerOffMethod {
	case shutdownCommand:
		s.Log("Shut down DUT")
		cmd := h.DUT.Conn().CommandContext(ctx, "/sbin/shutdown", "-P", "now")
		if err := cmd.Start(); err != nil {
			s.Fatal("Failed to shut down DUT: ", err)
		}
	case longPowerButtonPress:
		s.Log("Long press power button")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress); err != nil {
			s.Fatal("Failed to power off DUT with long press of the power button: ", err)
		}
	case powerStateOff:
		s.Log("Power state off")
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateOff); err != nil {
			s.Fatal("Failed to power off DUT with power state off: ", err)
		}
	}

	h.DisconnectDUT(ctx)
	s.Log("Check for G3 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3"); err != nil {
		s.Fatal("Failed to get G3 powerstate: ", err)
	}

	if tc.PowerOffMethod == powerStateOff {
		// Verify that power state off doesn't power back on by mistake
		s.Log("Power state off")
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateOff); err != nil {
			s.Fatal("Failed to power off DUT with power state off: ", err)
		}
		testing.Sleep(ctx, 10*time.Second)
		if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3"); err != nil {
			s.Fatal("Failed to get G3 powerstate: ", err)
		}
		s.Log("Power DUT back on with power state on")
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateOn); err != nil {
			s.Fatal("Failed to power on DUT with power state on: ", err)
		}
	} else {
		s.Log("Power DUT back on with short press of the power button")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
			s.Fatal("Failed to power on DUT with short press of the power button: ", err)
		}
	}

	if err := h.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT after restarting: ", err)
	}
	if tc.RemovePower {
		h.SetDUTPower(ctx, true)
		// Restoring power with servo_v4 can cause ethernet failure, so reconnect afterwards
		if err := h.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to reconnect to DUT after restarting: ", err)
		}
	}

	if h.DUT != nil {
		if bootMode, err := h.Reporter.CurrentBootMode(ctx); err != nil {
			s.Fatal("Failed to get boot mode: ", err)
		} else if bootMode != fwCommon.BootModeNormal {
			s.Fatal("Unexpected boot mode: ", bootMode)
		}
	}
}
