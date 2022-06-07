// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type consecutiveBootMethod int

const (
	consecutiveBootWithPowerBtn consecutiveBootMethod = iota
	consecutiveBootWithShutdownCmd
)

type argsForConsecutiveBoot struct {
	bootMethod consecutiveBootMethod
	bootMode   string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ConsecutiveBoot,
		Desc:         "Test DUT shuts down and boots to ChromeOS over many iterations",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Vars:         []string{"firmware.consecutiveBootIters"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Params: []testing.Param{
			{
				Name:    "power_button_normal_mode",
				Fixture: fixture.NormalMode,
				Timeout: 20 * time.Minute,
				Val: argsForConsecutiveBoot{
					bootMethod: consecutiveBootWithPowerBtn,
					bootMode:   "normal",
				},
			},
			{
				Name:    "power_button_dev_mode",
				Fixture: fixture.DevModeGBB,
				Timeout: 20 * time.Minute,
				Val: argsForConsecutiveBoot{
					bootMethod: consecutiveBootWithPowerBtn,
					bootMode:   "developer",
				},
			},
			{
				Name:    "shutdown_cmd_normal_mode",
				Fixture: fixture.NormalMode,
				Timeout: 20 * time.Minute,
				Val: argsForConsecutiveBoot{
					bootMethod: consecutiveBootWithShutdownCmd,
					bootMode:   "normal",
				},
			},
			{
				Name:    "shutdown_cmd_dev_mode",
				Fixture: fixture.DevModeGBB,
				Timeout: 20 * time.Minute,
				Val: argsForConsecutiveBoot{
					bootMethod: consecutiveBootWithShutdownCmd,
					bootMode:   "developer",
				},
			},
		},
	})
}

func ConsecutiveBoot(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	testArgs := s.Param().(argsForConsecutiveBoot)
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	numIters := 10
	if numItersStr, ok := s.Var("firmware.consecutiveBootIters"); ok {
		numItersInt, err := strconv.Atoi(numItersStr)
		if err != nil {
			s.Fatalf("Invalid value for var firmware.consecutiveBootIters: got %q, expected int", numItersStr)
		} else {
			numIters = numItersInt
		}
	}

	verifyBootMode := func(mode string) error {
		if mainfwType, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamMainfwType); err != nil {
			return errors.Wrap(err, "failed to get crossystem mainfw_type")
		} else if mainfwType != testArgs.bootMode {
			return errors.Errorf("expected mainfw_type to be %s, got %q", testArgs.bootMode, mainfwType)
		}
		return nil
	}

	shutdownWithPowerButton := func() {
		s.Log("Pressing power key until device shuts down")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
			s.Fatal("Failed to press power key: ", err)
		}
	}

	shutdownWithShutdownCmd := func() {
		s.Log("Sending `/sbin/shutdown -P now` to shutdown dut")
		if err := h.DUT.Conn().CommandContext(ctx, "/sbin/shutdown", "-P", "now").Start(); err != nil {
			s.Fatal("Failed to run `/sbin/shutdown -P now` cmd: ", err)
		}
	}

	var shutdownFunc func()
	if testArgs.bootMethod == consecutiveBootWithPowerBtn {
		shutdownFunc = shutdownWithPowerButton
	} else if testArgs.bootMethod == consecutiveBootWithShutdownCmd {
		shutdownFunc = shutdownWithShutdownCmd
	}

	s.Log("Verifying boot mode is ", testArgs.bootMode)
	if err := verifyBootMode(testArgs.bootMode); err != nil {
		s.Fatal("Failed boot mode check: ", err)
	}

	for i := 0; i < numIters; i++ {
		s.Logf("Running iteration %d out of %d ", i+1, numIters)

		bootID, err := h.Reporter.BootID(ctx)
		if err != nil {
			s.Fatal("Failed to get boot id: ", err)
		}

		shutdownFunc()

		s.Logf("Sleep for %s so shutdown completes", h.Config.Shutdown)
		if err := testing.Sleep(ctx, h.Config.Shutdown); err != nil {
			s.Fatal("Failed to sleep waiting for shutdown: ", err)
		}

		s.Log("Check for G3 powerstate")
		if err := h.WaitForPowerStates(ctx, 1*time.Second, 30*time.Second, "G3"); err != nil {
			s.Fatal("Failed to get G3 powerstate: ", err)
		}

		s.Log("Pressing power key until device boots")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOn)); err != nil {
			s.Fatal("Failed to press power key: ", err)
		}

		s.Log("Check for S0 powerstate")
		if err := h.WaitForPowerStates(ctx, 1*time.Second, 60*time.Second, "S0"); err != nil {
			s.Fatal("Failed to get S0 powerstate: ", err)
		}

		s.Log("Wait for DUT to connect")
		if err := h.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to wait for device to connect: ", err)
		}

		// Make sure boot mode is preserved over reboot.
		s.Log("Verifying boot mode is ", testArgs.bootMode)
		if err := verifyBootMode(testArgs.bootMode); err != nil {
			s.Fatal("Failed boot mode check: ", err)
		}

		s.Log("Verifying boot id changed over reboot")
		newBootID, err := h.Reporter.BootID(ctx)
		if err != nil {
			s.Fatal("Failed to get boot id: ", err)
		}
		if newBootID == bootID {
			s.Fatal("Unexpectedly got same boot id over reboot")
		}
	}
}
