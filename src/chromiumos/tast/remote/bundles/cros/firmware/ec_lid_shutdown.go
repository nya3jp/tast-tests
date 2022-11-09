// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECLidShutdown,
		Desc:         "Verify setting GBBFlag_DISABLE_LID_SHUTDOWN flag prevents shutdown with closed lid on fw screen",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Lid()),
		Fixture:      fixture.NormalMode,
		Timeout:      10 * time.Minute,
	})
}

func ECLidShutdown(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if err := h.Servo.OpenLid(ctx); err != nil {
		s.Fatal("Failed to re open lid: ", err)
	}

	s.Log(ctx, "Setting USB mux state to off to make sure it doesn't use USB for recovery")
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxOff); err != nil {
		s.Fatal("Failed to set usb mux state to off: ", err)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	if err := testWithFlag(ctx, h); err != nil {
		s.Fatal("Failed to power on and off correctly with GBBFlag_DISABLE_LID_SHUTDOWN set: ", err)
	}

	if err := testWithoutFlag(ctx, h); err != nil {
		s.Fatal("Failed to power on and off correctly with GBBFlag_DISABLE_LID_SHUTDOWN not set: ", err)
	}
}

func testWithFlag(ctx context.Context, h *firmware.Helper) (reterr error) {
	testing.ContextLog(ctx, "Setting GBBFlag_DISABLE_LID_SHUTDOWN flag")
	flags := pb.GBBFlagsState{Clear: []pb.GBBFlag{}, Set: []pb.GBBFlag{pb.GBBFlag_DISABLE_LID_SHUTDOWN}}
	if _, err := h.BiosServiceClient.ClearAndSetGBBFlags(ctx, &flags); err != nil {
		return errors.Wrap(err, "setting GBBFlag_DISABLE_LID_SHUTDOWN flag")
	}

	powerdCmd := "echo 0 > /var/lib/power_manager/use_lid"
	if out, err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", powerdCmd).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to set use_lid with output: %v", string(out))
	}
	defer func(ctx context.Context) {
		if err := h.Servo.OpenLid(ctx); err != nil {
			if reterr != nil {
				testing.ContextLog(ctx, "Failed to open lid after test: ", err)
			} else {
				reterr = errors.Wrap(err, "failed to re open lid")
			}
			return
		}

		testing.ContextLog(ctx, "Resetting DUT")
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
			if reterr != nil {
				testing.ContextLog(ctx, "Failed reset DUT after test: ", err)
			} else {
				reterr = errors.Wrap(err, "failed to reset DUT")
			}
			return
		}

		testing.ContextLog(ctx, "Waiting for DUT to connect")
		if err := h.WaitConnect(ctx); err != nil {
			if reterr != nil {
				testing.ContextLog(ctx, "Failed to connect to DUT after test: ", err)
			} else {
				reterr = errors.Wrap(err, "failed to connect to DUT")
			}
			return
		}

		// Next test case will clear gbb flag, so don't exit early if this fails.
		testing.ContextLog(ctx, "Clear GBBFlag_DISABLE_LID_SHUTDOWN flag")
		flags := pb.GBBFlagsState{Clear: []pb.GBBFlag{pb.GBBFlag_DISABLE_LID_SHUTDOWN}, Set: []pb.GBBFlag{}}
		if _, err := h.BiosServiceClient.ClearAndSetGBBFlags(ctx, &flags); err != nil {
			testing.ContextLog(ctx, "Failed clearing GBBFlag_DISABLE_LID_SHUTDOWN flag: ", err)
		}

		// Rebooting clears this, verify the var file no longer exists.
		testing.ContextLog(ctx, "Verifying use_lid setting was reset")
		if out, err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", "[ ! -f /var/lib/power_manager/use_lid ]").CombinedOutput(ssh.DumpLogOnError); err != nil {
			if reterr != nil {
				testing.ContextLogf(ctx, "Failed to restore use_lid settings with output: %v: %v", string(out), err)
			} else {
				reterr = errors.Wrapf(err, "failed to restore use_lid settings with output: %v", string(out))
			}
			return
		}
	}(ctx)

	testing.ContextLog(ctx, "Booting to recovery")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
		return errors.Wrap(err, "powering off DUT")
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "failed to close lid")
	}

	// Immediately checking for S0 might cause a false positive since
	// its already in S0 it might not have had time to go to G3.
	testing.ContextLog(ctx, "Sleep so lid close has time to affect power_state")
	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep for 10s")
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	return nil
}

func testWithoutFlag(ctx context.Context, h *firmware.Helper) (reterr error) {
	testing.ContextLog(ctx, "Clearing GBBFlag_DISABLE_LID_SHUTDOWN flag")
	flags := pb.GBBFlagsState{Clear: []pb.GBBFlag{pb.GBBFlag_DISABLE_LID_SHUTDOWN}, Set: []pb.GBBFlag{}}
	if _, err := h.BiosServiceClient.ClearAndSetGBBFlags(ctx, &flags); err != nil {
		return errors.Wrap(err, "clearing GBBFlag_DISABLE_LID_SHUTDOWN flag")
	}

	defer func(ctx context.Context) {
		if err := h.Servo.OpenLid(ctx); err != nil {
			if reterr != nil {
				testing.ContextLog(ctx, "Failed to open lid after test: ", err)
			} else {
				reterr = errors.Wrap(err, "failed to re open lid")
			}
			return
		}

		testing.ContextLog(ctx, "Resetting DUT")
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
			if reterr != nil {
				testing.ContextLog(ctx, "Failed reset DUT after test: ", err)
			} else {
				reterr = errors.Wrap(err, "failed to reset DUT")
			}
			return
		}

		testing.ContextLog(ctx, "Waiting for DUT to connect")
		if err := h.WaitConnect(ctx); err != nil {
			if reterr != nil {
				testing.ContextLog(ctx, "Failed to connect to DUT after test: ", err)
			} else {
				reterr = errors.Wrap(err, "failed to connect to DUT")
			}
			return
		}
	}(ctx)

	testing.ContextLog(ctx, "Booting to recovery")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
		return errors.Wrap(err, "powering off DUT")
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "failed to close lid")
	}

	testing.ContextLog(ctx, "Waiting for G3 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3"); err != nil {
		return errors.Wrap(err, "failed to get G3 powerstate")
	}

	return nil
}
