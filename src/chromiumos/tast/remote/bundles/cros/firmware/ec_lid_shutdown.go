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
		Timeout:      15 * time.Minute,
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

	s.Log("\tSetting USB mux state to off to make sure it doesn't use USB for recovery")
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxOff); err != nil {
		s.Fatal("Failed to set usb mux state to off: ", err)
	}

	defer func() {
		s.Log("Resetting DUT after test")
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
			s.Fatal("Failed to reset DUT: ", err)
		}

		s.Log("Reconnecting to DUT")
		h.DisconnectDUT(ctx)
		if err := h.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to connect to DUT: ", err)
		}
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Failed to connect to the bios service on the DUT: ", err)
		}

		s.Log("Clear GBBFlag_DISABLE_LID_SHUTDOWN flag after test end")
		flags := pb.GBBFlagsState{Clear: []pb.GBBFlag{pb.GBBFlag_DISABLE_LID_SHUTDOWN}, Set: []pb.GBBFlag{}}
		if _, err := h.BiosServiceClient.ClearAndSetGBBFlags(ctx, &flags); err != nil {
			s.Fatal("Failed to clear GBBFlag_DISABLE_LID_SHUTDOWN flag after test end: ", err)
		}
	}()

	s.Log("\tSet flag then go to recovery mode, expect S0 after lid close")
	if err := setFlagBeforeRecMode(ctx, h, true); err != nil {
		s.Fatal("Failed to power on and off correctly with GBBFlag_DISABLE_LID_SHUTDOWN set: ", err)
	}

	s.Log("\tClear flag then go to recovery mode, expect G3 after lid close")
	if err := setFlagBeforeRecMode(ctx, h, false); err != nil {
		s.Fatal("Failed to power on and off correctly with GBBFlag_DISABLE_LID_SHUTDOWN not set: ", err)
	}

	s.Log("\tSet flag then power off, go to recovery mode, expect S0 after lid close")
	if err := setFlagBeforePowerOff(ctx, h, true); err != nil {
		s.Fatal("Failed to power on and off correctly with GBBFlag_DISABLE_LID_SHUTDOWN not set: ", err)
	}

	s.Log("\tClear flag then power off, go to recovery mode, expect G3 after lid close")
	if err := setFlagBeforePowerOff(ctx, h, false); err != nil {
		s.Fatal("Failed to power on and off correctly with GBBFlag_DISABLE_LID_SHUTDOWN not set: ", err)
	}
}

func setFlagBeforePowerOff(ctx context.Context, h *firmware.Helper, flag bool) (reterr error) {
	testing.ContextLog(ctx, "Resetting DUT")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
		return errors.Wrap(err, "failed to reset DUT")
	}
	h.DisconnectDUT(ctx)
	testing.ContextLog(ctx, "Reconnect to DUT")
	if err := h.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to DUT")
	}
	if err := h.RequireBiosServiceClient(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to the bios service on the DUT")
	}

	flags := pb.GBBFlagsState{Clear: []pb.GBBFlag{pb.GBBFlag_DISABLE_LID_SHUTDOWN}, Set: []pb.GBBFlag{}}
	flagState := "clearing"
	if flag {
		flags = pb.GBBFlagsState{Clear: []pb.GBBFlag{}, Set: []pb.GBBFlag{pb.GBBFlag_DISABLE_LID_SHUTDOWN}}
		flagState = "setting"
	}
	testing.ContextLogf(ctx, "%s GBBFlag_DISABLE_LID_SHUTDOWN flag", flagState)
	if _, err := h.BiosServiceClient.ClearAndSetGBBFlags(ctx, &flags); err != nil {
		return errors.Wrapf(err, "failed %s GBBFlag_DISABLE_LID_SHUTDOWN flag", flagState)
	}

	testing.ContextLog(ctx, "Powering off DUT")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateOff); err != nil {
		return errors.Wrap(err, "powering off DUT")
	}

	testing.ContextLog(ctx, "Booting to recovery")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
		return errors.Wrap(err, "powering off DUT")
	}

	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "failed to close lid")
	}

	expectedPowerState := "G3"
	if flag {
		expectedPowerState = "S0"
	}
	testing.ContextLogf(ctx, "Waiting for %s powerstate", expectedPowerState)
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, expectedPowerState); err != nil {
		return errors.Wrapf(err, "failed to get %s powerstate", expectedPowerState)
	}

	return nil
}

func setFlagBeforeRecMode(ctx context.Context, h *firmware.Helper, flag bool) (reterr error) {
	testing.ContextLog(ctx, "Resetting DUT")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
		return errors.Wrap(err, "failed to reset DUT")
	}
	h.DisconnectDUT(ctx)
	testing.ContextLog(ctx, "Reconnect to DUT")
	if err := h.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to DUT")
	}
	if err := h.RequireBiosServiceClient(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to the bios service on the DUT")
	}

	flags := pb.GBBFlagsState{Clear: []pb.GBBFlag{pb.GBBFlag_DISABLE_LID_SHUTDOWN}, Set: []pb.GBBFlag{}}
	flagState := "clearing"
	if flag {
		flags = pb.GBBFlagsState{Clear: []pb.GBBFlag{}, Set: []pb.GBBFlag{pb.GBBFlag_DISABLE_LID_SHUTDOWN}}
		flagState = "setting"
	}
	testing.ContextLogf(ctx, "%s GBBFlag_DISABLE_LID_SHUTDOWN flag", flagState)
	if _, err := h.BiosServiceClient.ClearAndSetGBBFlags(ctx, &flags); err != nil {
		return errors.Wrapf(err, "failed %s GBBFlag_DISABLE_LID_SHUTDOWN flag", flagState)
	}

	testing.ContextLog(ctx, "Booting to recovery")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
		return errors.Wrap(err, "powering off DUT")
	}

	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "failed to close lid")
	}

	// Immediately checking for powerstate might cause a false positive since it might not have time to transition.
	testing.ContextLog(ctx, "Sleep so lid close has time to affect power_state")
	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep for 10s")
	}

	expectedPowerState := "G3"
	if flag {
		expectedPowerState = "S0"
	}
	testing.ContextLogf(ctx, "Waiting for %s powerstate", expectedPowerState)
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, expectedPowerState); err != nil {
		return errors.Wrapf(err, "failed to get %s powerstate", expectedPowerState)
	}

	return nil
}
