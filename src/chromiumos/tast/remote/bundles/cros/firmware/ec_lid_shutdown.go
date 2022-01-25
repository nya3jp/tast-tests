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
		Fixture:      fixture.DevMode,
		Timeout:      3 * time.Minute,
	})
}

func ECLidShutdown(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	testing.ContextLog(ctx, "stopping powerd")
	if _, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", "echo 0 > /var/lib/power_manager/use_lid").Output(ssh.DumpLogOnError); err != nil {
		s.Fatal("failed to set ignore lid state: ", err)
	}
	defer func() {
		testing.ContextLog(ctx, "stopping powerd")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", "/var/lib/power_manager/use_lid").Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("failed to unset ignore lid state: ", err)
		}

	}()

	// if err := testWithoutFlag(ctx, h, s.DUT()); err != nil {
	// 	s.Fatal("Failed to power on and off correctly with GBBFlag_DISABLE_LID_SHUTDOWN not set: ", err)
	// }

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

	testing.ContextLog(ctx, "Waiting for S0 powerstate to make sure DUT is on fw screen")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "failed to close lid")
	}

	testing.ContextLog(ctx, "Expect S5/G3 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S5", "G3"); err != nil {
		return errors.Wrap(err, "failed to get S5/G3 powerstate")
	}

	testing.ContextLog(ctx, "Go from fw screen to normal mode")
	if err := ms.FwScreenToNormalMode(ctx, false, false); err != nil {
		return errors.Wrap(err, "failed to go to normal mode from fw screen")
	}

	testing.ContextLog(ctx, "Sleeping for 15s to make sure DUT has time to reach powerstate")
	if err := testing.Sleep(ctx, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
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

	testing.ContextLog(ctx, "Setting GBBFlag_DISABLE_LID_SHUTDOWN flag")
	flags := pb.GBBFlagsState{Clear: []pb.GBBFlag{}, Set: []pb.GBBFlag{pb.GBBFlag_DISABLE_LID_SHUTDOWN}}
	if err := common.ClearAndSetGBBFlags(ctx, d, &flags); err != nil {
		return errors.Wrap(err, "setting GBBFlag_DISABLE_LID_SHUTDOWN flag")
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}

	// testing.ContextLog(ctx, "Setting USB mux state to off")
	// if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxOff); err != nil {
	// 	return errors.Wrap(err, "setting usb mux state to off while DUT is off")
	// }

	// testing.ContextLog(ctx, "Sleeping for 1s to make sure USB mux state is set")
	// if err := testing.Sleep(ctx, 1*time.Second); err != nil {
	// 	return errors.Wrap(err, "failed to sleep")
	// }

	// if err := ms.PowerOff(ctx); err != nil {
	// 	return errors.Wrap(err, "powering off DUT")
	// }

	// testing.ContextLog(ctx, "stopping powerd")
	// if _, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", "echo 0 > /var/lib/power_manager/use_lid").Output(ssh.DumpLogOnError); err != nil {
	// 	return errors.Wrap(err, "failed to set ignore lid state")
	// }

	// testing.ContextLog(ctx, "Disabling dev_boot_usb, disabling dev_boot_signed_only")
	// if err := h.DUT.Conn().CommandContext(ctx, "crossystem", "recovery_request=1").Run(ssh.DumpLogOnError); err != nil {
	// 	return errors.Wrap(err, "disabling dev_boot_usb")
	// }

	// if err := h.Servo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
	// 	return errors.Wrapf(err, "setting power state to %s", servo.PowerStateRec)
	// }

	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "failed to close lid")
	}

	testing.ContextLog(ctx, "Sleeping for 5s to make sure DUT has time to reach powerstate")
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	// if err := h.Servo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
	// 	return errors.Wrapf(err, "setting power state to %s", servo.PowerStateRec)
	// }

	// testing.ContextLog(ctx, "Setting power state to rec")
	// if err := ms.RebootToMode(ctx, common.BootModeRecovery, firmware.AllowGBBForce, firmware.AssumeGBBFlagsCorrect); err != nil {
	// 	return errors.Wrapf(err, "setting power state to %s", servo.PowerStateRec)
	// }

	testing.ContextLog(ctx, "Waiting for S0 powerstate to make sure DUT is on fw screen")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	// if err := h.Servo.CloseLid(ctx); err != nil {
	// 	return errors.Wrap(err, "failed to close lid")
	// }

	// testing.ContextLog(ctx, "Expect S0 powerstate")
	// if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
	// 	return errors.Wrap(err, "failed to get S0 powerstate")
	// }

	// if err := h.Servo.SetOnOff(ctx, servo.USBKeyboard, servo.On); err != nil {
	// 	return errors.Wrap(err, "Failed to enable usb keyboard")
	// }

	// testing.ContextLog(ctx, "Go from fw screen to normal mode")
	// if err := ms.FwScreenToNormalMode(ctx, false, false); err != nil {
	// 	return errors.Wrap(err, "unexpectedly booted to normal mode")
	// }

	testing.ContextLog(ctx, "Go from fw screen to normal mode")
	if err := ms.RebootToMode(ctx, common.BootModeDev, firmware.AssumeGBBFlagsCorrect); err != nil {
		return errors.Wrap(err, "unexpectedly booted to normal mode")
	}

	// testing.ContextLog(ctx, "Getting current bootmode")
	// if currMode, err := h.Reporter.CurrentBootMode(ctx); err != nil {
	// 	return errors.Wrap(err, "failed to get current boot mode")
	// } else if currMode != common.BootModeNormal {
	// 	return errors.Errorf("expected boot mode: %v, actual mode: %v", common.BootModeNormal, currMode)
	// }

	// testing.ContextLog(ctx, "Sleeping for 15s to make sure DUT has time to reach powerstate")
	// if err := testing.Sleep(ctx, 15*time.Second); err != nil {
	// 	return errors.Wrap(err, "failed to sleep")
	// }

	// testing.ContextLog(ctx, "Expect S0 powerstate")
	// if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
	// 	return errors.Wrap(err, "failed to get S0 powerstate")
	// }

	// testing.ContextLog(ctx, "Getting current bootmode")
	// if currMode, err := h.Reporter.CurrentBootMode(ctx); err != nil {
	// 	return errors.Wrap(err, "failed to get current boot mode")
	// } else if currMode != common.BootModeNormal {
	// 	return errors.Errorf("expected boot mode: %v, actual mode: %v", common.BootModeNormal, currMode)
	// }

	// if err := h.Servo.OpenLid(ctx); err != nil {
	// 	return errors.Wrap(err, "failed to re open lid")
	// }

	// testing.ContextLog(ctx, "Expect S0 powerstate")
	// if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
	// 	return errors.Wrap(err, "failed to get S0 powerstate")
	// }

	// ms, err = firmware.NewModeSwitcher(ctx, h)
	// if err != nil {
	// 	return errors.Wrap(err, "failed to create mode switcher")
	// }

	// testing.ContextLog(ctx, "Go from fw screen to normal mode")
	// if err := ms.FwScreenToNormalMode(ctx, false, false); err != nil {
	// 	return errors.Wrap(err, "unexpectedly booted to normal mode")
	// }

	testing.ContextLog(ctx, "Getting current bootmode")
	if currMode, err := h.Reporter.CurrentBootMode(ctx); err != nil {
		return errors.Wrap(err, "failed to get current boot mode")
	} else if currMode != common.BootModeNormal && currMode != common.BootModeDev {
		testing.ContextLog(ctx, "Current Boot mode: ", currMode)
		return errors.Errorf("expected boot mode: %v, actual mode: %v", common.BootModeNormal, currMode)
	} else {
		testing.ContextLog(ctx, "Current Boot mode: ", currMode)
	}

	// return nil

	// testing.ContextLog(ctx, "Waiting for DUT to connect")
	// return h.WaitConnect(ctx)

	// testing.ContextLog(ctx, "Sleeping for 1s to make sure USB mux state is set")
	// if err := testing.Sleep(ctx, 1*time.Second); err != nil {
	// 	return errors.Wrap(err, "failed to sleep")
	// }

	// testing.ContextLog(ctx, "Waiting for S0 powerstate to make sure DUT is on fw screen")
	// if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
	// 	return errors.Wrap(err, "failed to get S0 powerstate")
	// }

	// // testing.ContextLog(ctx, "Disabling dev_boot_usb, disabling dev_boot_signed_only")
	// // if err := h.DUT.Conn().CommandContext(ctx, "crossystem", "dev_boot_usb=0", "dev_boot_signed_only=0", "dev_default_boot=disk").Run(ssh.DumpLogOnError); err != nil {
	// // 	return errors.Wrap(err, "disabling dev_boot_usb")
	// // }

	// testing.ContextLog(ctx, "Sleeping for 1s to make sure USB mux state is set")
	// if err := testing.Sleep(ctx, 1*time.Second); err != nil {
	// 	return errors.Wrap(err, "failed to sleep")
	// }

	// // if err := ms.PowerOff(ctx); err != nil {
	// // 	return errors.Wrap(err, "powering off DUT")
	// // }

	// if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
	// 	return errors.Wrapf(err, "setting power state to %s", servo.PowerStateReset)
	// }

	// // testing.ContextLog(ctx, "stopping powerd")
	// // if _, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", "echo 0 > /var/lib/power_manager/use_lid").Output(ssh.DumpLogOnError); err != nil {
	// // 	return errors.Wrap(err, "failed to set ignore lid state")
	// // }

	// // testing.ContextLog(ctx, "Sleeping for 3s to make sure fw screen was reached")
	// // if err := testing.Sleep(ctx, 10*time.Second); err != nil {
	// // 	return errors.Wrap(err, "failed to sleep")
	// // }

	// // testing.ContextLog(ctx, "Waiting for S0 powerstate to make sure DUT is on fw screen")
	// // if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
	// // 	return errors.Wrap(err, "failed to get S0 powerstate")
	// // }

	// // if err := h.Servo.CloseLid(ctx); err != nil {
	// // 	return errors.Wrap(err, "failed to close lid")
	// // }

	// // defer func() error {
	// // testing.ContextLog(ctx, "Resetting dut")
	// // if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
	// // 	return errors.Wrapf(err, "failed to set power state to %s", servo.PowerStateReset)
	// // }

	// // // [ ! -f /var/lib/power_manager/use_lid ]
	// // testing.ContextLog("Verifying ignore lid state is no longer active")
	// // if _, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", "[ ! -f /var/lib/power_manager/use_lid ]").Output(ssh.DumpLogOnError); err != nil {
	// // 	return errors.Wrap(err, "failed to unset ignore lid state")
	// // }

	// // }()

	// // testing.ContextLog(ctx, "Sleeping for 5s to make sure DUT has time to reach powerstate")
	// // if err := testing.Sleep(ctx, 5*time.Second); err != nil {
	// // 	return errors.Wrap(err, "failed to sleep")
	// // }

	// // testing.ContextLog(ctx, "Expect S0 powerstate")
	// // if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
	// // 	return errors.Wrap(err, "failed to get S0 powerstate")
	// // }

	// testing.ContextLog(ctx, "Go from fw screen to dev mode")
	// if err := ms.FwScreenToDevMode(ctx, false, false); err != nil {
	// 	return errors.Wrap(err, "failed to boot to go to dev mode")
	// }

	// // testing.ContextLog(ctx, "Checking current boot mode")
	// // if err := testing.Poll(ctx, func(ctx context.Context) error {
	// // 	if currMode, err := h.Reporter.CurrentBootMode(ctx); err != nil {
	// // 		return errors.Wrap(err, "failed to get current boot mode")
	// // 	} else if currMode != common.BootModeNormal {
	// // 		return errors.Errorf("expected boot mode: %v, actual mode: %b", common.BootModeNormal, currMode)
	// // 	}
	// // 	return nil
	// // }, &testing.PollOptions{Timeout: 1 * time.Minute}); err != nil {
	// // 	return errors.Wrap(err, "failed to poll for boot mode")
	// // }

	// if currMode, err := h.Reporter.CurrentBootMode(ctx); err != nil {
	// 	return errors.Wrap(err, "failed to get current boot mode")
	// } else if currMode != common.BootModeDev {
	// 	return errors.Errorf("expected boot mode: %v, actual mode: %v", common.BootModeNormal, currMode)
	// }

	// testing.ContextLog(ctx, "Expect S0 powerstate")
	// if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
	// 	return errors.Wrap(err, "failed to get S0 powerstate")
	// }

	if err := h.Servo.OpenLid(ctx); err != nil {
		return errors.Wrap(err, "failed to re open lid")
	}

	testing.ContextLog(ctx, "Expect S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	testing.ContextLog(ctx, "Resetting dut")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
		return errors.Wrapf(err, "failed to set power state to %s", servo.PowerStateReset)
	}

	// // [ ! -f /var/lib/power_manager/use_lid ]
	// testing.ContextLog(ctx, "Verifying ignore lid state is no longer active")
	// if _, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", "[ ! -f /var/lib/power_manager/use_lid ]").Output(ssh.DumpLogOnError); err != nil {
	// 	return errors.Wrap(err, "failed to unset ignore lid state")
	// }

	testing.ContextLog(ctx, "Setting GBBFlag_DISABLE_LID_SHUTDOWN flag")
	if err := common.ClearAndSetGBBFlags(ctx, d, &pb.GBBFlagsState{Clear: []pb.GBBFlag{pb.GBBFlag_DISABLE_LID_SHUTDOWN}, Set: []pb.GBBFlag{}}); err != nil {
		return errors.Wrap(err, "setting GBBFlag_DISABLE_LID_SHUTDOWN flag")
	}

	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "failed to close lid")
	}

	testing.ContextLog(ctx, "Sleeping for 15s to make sure DUT has time to reach powerstate")
	if err := testing.Sleep(ctx, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	testing.ContextLog(ctx, "Waiting for S5 powerstate to make sure DUT is on fw screen")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S5"); err != nil {
		return errors.Wrap(err, "failed to get S5 powerstate")
	}

	if err := h.Servo.OpenLid(ctx); err != nil {
		return errors.Wrap(err, "failed to re open lid")
	}

	testing.ContextLog(ctx, "Expect S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	testing.ContextLog(ctx, "Resetting dut")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
		return errors.Wrapf(err, "failed to set power state to %s", servo.PowerStateReset)
	}

	return nil
}
