// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type powerOffTest int

const (
	powerOffWithAndWithoutPowerd powerOffTest = iota
	ignoresShortPowerKey
	powerOffWithShortPowerKey
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECPowerButton,
		Desc:         "Verify using servo power key results in expected shutdown behaviour",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware"},
		SoftwareDeps: []string{"crossystem"},
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Fixture:      fixture.NormalMode,
		Timeout:      45 * time.Minute,
		Params: []testing.Param{
			{
				Name:      "toggle_powerd",
				Val:       powerOffWithAndWithoutPowerd,
				ExtraAttr: []string{"firmware_unstable"},
			},
			{
				Name:              "ignore_short_power_key",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				Val:               ignoresShortPowerKey,
				ExtraAttr:         []string{"firmware_unstable"},
			},
			{
				Name:              "short_power_key",
				ExtraHardwareDeps: hwdep.D(hwdep.NoInternalDisplay()),
				Val:               powerOffWithShortPowerKey,
				ExtraAttr:         []string{"firmware_unstable"},
			},
		},
	})
}

const (
	shortPowerKeyPressDur time.Duration = 200 * time.Millisecond
)

func ECPowerButton(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to require config: ", err)
	}

	switch s.Param().(powerOffTest) {
	case powerOffWithAndWithoutPowerd:
		s.Log("Test power off with power state")
		if err := testRebootWithSettingPowerState(ctx, h); err != nil {
			s.Fatal("Failed to reboot from setting servo powerstate: ", err)
		}
		s.Log("Test power off with powerd on and off")
		if err := testPowerdPowerOff(ctx, h); err != nil {
			s.Fatal("Failed powering off with or without powerd: ", err)
		}
	case ignoresShortPowerKey:
		// If DUT has internal display, expect 200ms power key press to be ignored.
		s.Log("Test that device with internal display ignores short power key press")
		if err := testIgnoreShortPowerKey(ctx, h); err != nil {
			s.Fatal("DUT unexpectedly shut down from short power key press: ", err)
		}
	case powerOffWithShortPowerKey:
		// If DUT doesn't have internal display, expect 200ms power key press to power off DUT.
		s.Log("Test device without internal display doesn't ignore short power key press")
		if err := testPowerOffWithShortPowerKey(ctx, h); err != nil {
			s.Fatal("DUT didn't shut down from short power key press: ", err)
		}
	}
}

func testPowerOffWithShortPowerKey(ctx context.Context, h *firmware.Helper) error {
	testing.ContextLog(ctx, "Pressing power key")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(shortPowerKeyPressDur)); err != nil {
		return errors.Wrap(err, "failed to press power key on DUT")
	}
	h.DisconnectDUT(ctx)

	testing.ContextLog(ctx, "Checking for S5 or G3 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S5", "G3"); err != nil {
		return errors.Wrap(err, "failed to get S5 or G3 powerstate")
	}

	testing.ContextLog(ctx, "Pressing power key")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOn)); err != nil {
		return errors.Wrap(err, "failed to press power key on DUT")
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	return h.WaitConnect(ctx)
}

func testIgnoreShortPowerKey(ctx context.Context, h *firmware.Helper) error {
	testing.ContextLog(ctx, "Getting current boot id")
	bootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	testing.ContextLog(ctx, "Pressing power key")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(shortPowerKeyPressDur)); err != nil {
		return errors.Wrap(err, "failed to press power key on DUT")
	}

	if err := testing.Sleep(ctx, shortPowerKeyPressDur); err != nil {
		return errors.Wrapf(err, "failed to sleep for %s ms", shortPowerKeyPressDur)
	}

	testing.ContextLog(ctx, "Expect DUT to remain in S0 powerstate")
	if currPowerState, err := h.Servo.GetECSystemPowerState(ctx); err != nil {
		return errors.Wrap(err, "failed to get current power state")
	} else if currPowerState != "S0" {
		return errors.Errorf("Current power state is: %s, expected S0", currPowerState)
	}

	testing.ContextLog(ctx, "After short sleep (5s) expect DUT to still remain in S0")
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	if currPowerState, err := h.Servo.GetECSystemPowerState(ctx); err != nil {
		return errors.Wrap(err, "failed to get current power state")
	} else if currPowerState != "S0" {
		return errors.Errorf("Current power state is: %s, expected S0", currPowerState)
	}

	testing.ContextLog(ctx, "Get new boot id, compare to old")
	if newBootID, err := h.Reporter.BootID(ctx); err != nil {
		return errors.Wrap(err, "failed to get current boot id")
	} else if newBootID != bootID {
		return errors.Errorf("boot ID unexpectedly changed from %s to %s", bootID, newBootID)
	}
	return nil
}

func shutdownAndWake(ctx context.Context, h *firmware.Helper, shutDownDur time.Duration, expStates ...string) error {
	testing.ContextLogf(ctx, "Pressing power key for %s", shutDownDur)
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(shutDownDur)); err != nil {
		return errors.Wrap(err, "failed to press power key on DUT")
	}
	h.DisconnectDUT(ctx)

	testing.ContextLogf(ctx, "Checking for %v powerstates", expStates)
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, expStates...); err != nil {
		return errors.Wrapf(err, "failed to get %v powerstates", expStates)
	}

	// If we are expecting S5/G3, we might still get to G3 after S5, so give it a little time before we wake up again.
	if err := testing.Sleep(ctx, time.Second*2); err != nil {
		return errors.Wrap(err, "sleep failed")
	}

	testing.ContextLog(ctx, "Send cmd to EC to wake up from deepsleep")
	h.Servo.RunECCommand(ctx, "help")

	testing.ContextLog(ctx, "Pressing power key (press)")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Wrap(err, "failed to press power key on DUT")
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	testing.ContextLog(ctx, "Waiting for DUT to connect")
	if err := h.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for DUT to connect")
	}

	return nil
}

func enablePowerd(ctx context.Context, h *firmware.Helper, status bool) error {
	startOrStop := "start"
	if !status {
		startOrStop = "stop"
	}

	startStopJob := func(ctx context.Context, job string) error {
		cmd := h.DUT.Conn().CommandContext(ctx, startOrStop, job)
		stderr, _ := cmd.StderrPipe()
		if err := cmd.Start(); err != nil {
			return errors.Wrapf(err, "failed to run '%s %s' cmd on DUT", startOrStop, job)
		}
		scanner := bufio.NewScanner(stderr)
		errMsg := ""
		for scanner.Scan() {
			errMsg = fmt.Sprintf("%s\n%s", errMsg, scanner.Text())
		}
		if err := cmd.Wait(); err != nil && !strings.Contains(errMsg, "Job is already running") {
			return errors.Wrapf(err, "failed to %s job %v, got error: %s", startOrStop, job, errMsg)
		}
		return nil
	}

	if status {
		if err := startStopJob(ctx, "powerd"); err != nil {
			return errors.Wrap(err, "failed to start powerd")
		}
	}

	if err := startStopJob(ctx, "fwupd"); err != nil {
		return errors.Wrapf(err, "failed to %v fwupd", startOrStop)
	}

	if !status {
		if err := startStopJob(ctx, "powerd"); err != nil {
			return errors.Wrap(err, "failed to stop powerd")
		}
	}

	return nil
}

func testPowerdPowerOff(ctx context.Context, h *firmware.Helper) (reterr error) {

	// Make sure fwupd and powerd are running again after test.
	defer func() {
		if err := h.EnsureDUTBooted(ctx); err != nil {
			if reterr != nil {
				testing.ContextLog(ctx, "Failed to ensure dut booted: ", err)
			} else {
				reterr = errors.Wrap(err, "failed to ensure dut booted")
			}
			return
		}
		if err := enablePowerd(ctx, h, true); err != nil {
			if reterr != nil {
				testing.ContextLog(ctx, "Failed to restart powerd after test end: ", err)
			} else {
				reterr = errors.Wrap(err, "failed to restart powerd after test end")
			}
			return
		}
	}()

	powerdDur := h.Config.HoldPwrButtonPowerOff
	noPowerdDur := h.Config.HoldPwrButtonNoPowerdShutdown

	testing.ContextLog(ctx, "starting powerd")
	if err := enablePowerd(ctx, h, true); err != nil {
		return errors.Wrap(err, "failed to start powerd")
	}

	if err := shutdownAndWake(ctx, h, powerdDur, "S5", "G3"); err != nil {
		return errors.Wrap(err, "failed shut down and wake with powerd")
	}

	testing.ContextLog(ctx, "stopping powerd")
	if err := enablePowerd(ctx, h, false); err != nil {
		return errors.Wrap(err, "failed to stop powerd")
	}

	if err := shutdownAndWake(ctx, h, noPowerdDur, "G3"); err != nil {
		return errors.Wrap(err, "failed shut down and wake with no powerd")
	}
	return nil
}

func testRebootWithSettingPowerState(ctx context.Context, h *firmware.Helper) error {
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateOff); err != nil {
		return errors.Wrap(err, "failed to set 'power_state' to 'off'")
	}
	h.DisconnectDUT(ctx)

	testing.ContextLog(ctx, "Waiting for G3 or S5 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5"); err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}

	testing.ContextLog(ctx, "Pressing power key to turn on DUT")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOn)); err != nil {
		return errors.Wrap(err, "failed to press power key on DUT")
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	return h.WaitConnect(ctx)
}
