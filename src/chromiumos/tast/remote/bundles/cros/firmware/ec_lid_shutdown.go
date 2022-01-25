// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	common "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
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
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Lid()),
		SoftwareDeps: []string{"crossystem", "flashrom"},
		Fixture:      fixture.NormalMode,
		Timeout:      10 * time.Minute,
	})
}

func ECLidShutdown(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if err := testWithoutFlag(ctx, h, s.DUT()); err != nil {
		s.Fatal("Failed to power on and off correctly with GBBFlag_DISABLE_LID_SHUTDOWN not set: ", err)
	}

	if err := testWithFlag(ctx, h, s.DUT()); err != nil {
		s.Fatal("Failed to power on and off correctly with GBBFlag_DISABLE_LID_SHUTDOWN set: ", err)
	}
}

func testWithoutFlag(ctx context.Context, h *firmware.Helper, d *dut.DUT) error {
	testing.ContextLog(ctx, "Clearing GBBFlag_DISABLE_LID_SHUTDOWN flag")
	flags := pb.GBBFlagsState{Clear: []pb.GBBFlag{pb.GBBFlag_DISABLE_LID_SHUTDOWN}, Set: []pb.GBBFlag{}}
	if err := common.ClearAndSetGBBFlags(ctx, d, &flags); err != nil {
		return errors.Wrap(err, "clearing GBBFlag_DISABLE_LID_SHUTDOWN flag")
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}

	testing.ContextLog(ctx, "Setting USB mux state to off")
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxOff); err != nil {
		return errors.Wrap(err, "setting usb mux state to off while DUT is off")
	}

	testing.ContextLog(ctx, "Sleeping for 1s to make sure USB mux state is set")
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	if err := ms.PowerOff(ctx); err != nil {
		return errors.Wrap(err, "powering off DUT")
	}

	testing.ContextLog(ctx, "Setting power state to rec")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
		return errors.Wrapf(err, "setting power state to %s", servo.PowerStateRec)
	}

	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "failed to close lid")
	}

	testing.ContextLog(ctx, "Expect S5/G3 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S5", "G3"); err != nil {
		return errors.Wrap(err, "failed to get S5/G3 powerstate")
	}

	if err := h.Servo.OpenLid(ctx); err != nil {
		return errors.Wrap(err, "failed to re open lid")
	}

	testing.ContextLog(ctx, "Expect S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	testing.ContextLog(ctx, "Waiting for DUT to connect")
	return h.WaitConnect(ctx)
}

func testWithFlag(ctx context.Context, h *firmware.Helper, d *dut.DUT) error {
	testing.ContextLog(ctx, "Clearing GBBFlag_DISABLE_LID_SHUTDOWN flag")
	flags := pb.GBBFlagsState{Clear: []pb.GBBFlag{}, Set: []pb.GBBFlag{pb.GBBFlag_DISABLE_LID_SHUTDOWN}}
	if err := common.ClearAndSetGBBFlags(ctx, d, &flags); err != nil {
		return errors.Wrap(err, "clearing GBBFlag_DISABLE_LID_SHUTDOWN flag")
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}

	testing.ContextLog(ctx, "Disabling dev_boot_usb, disabling dev_boot_signed_only")
	if err := h.DUT.Conn().CommandContext(ctx, "crossystem", "dev_boot_usb=0", "dev_boot_signed_only=0", "dev_default_boot=disk").Run(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "disabling dev_boot_usb")
	}

	testing.ContextLog(ctx, "Setting USB mux state to off")
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxOff); err != nil {
		return errors.Wrap(err, "setting usb mux state to off while DUT is off")
	}

	testing.ContextLog(ctx, "Sleeping for 1s to make sure USB mux state is set")
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	if err := ms.PowerOff(ctx); err != nil {
		return errors.Wrap(err, "powering off DUT")
	}

	testing.ContextLog(ctx, "Setting power state to rec")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
		return errors.Wrapf(err, "setting power state to %s", servo.PowerStateRec)
	}

	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "failed to close lid")
	}

	testing.ContextLog(ctx, "Expect S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	testing.ContextLog(ctx, "Go from fw screen to normal mode")
	if err := ms.FwScreenToNormalMode(ctx, false, false); err != nil {
		return errors.Wrap(err, "failed to boot to go to normal mode")
	}

	testing.ContextLog(ctx, "Expect S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	if err := h.Servo.OpenLid(ctx); err != nil {
		return errors.Wrap(err, "failed to re open lid")
	}

	testing.ContextLog(ctx, "Expect S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	testing.ContextLog(ctx, "Waiting for DUT to connect")
	return h.WaitConnect(ctx)
}
