// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bufio"
	"context"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type powerOffTest int

const (
	powerOffWithAndWithoutPowerd powerOffTest = iota
	powerOffWithPowerKey
	ignoresShortPowerKey
	powerOffWithShortPowerKey
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECPowerButton,
		Desc:         "Verify enabling and disabling write protect works as expected",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Fixture:      fixture.NormalMode,
		Timeout:      10 * time.Minute,
		Params: []testing.Param{
			{
				Name: "toggle_powerd",
				Val:  powerOffWithAndWithoutPowerd,
			},
			{
				Name:              "servo_power_key",
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnFormFactor(hwdep.Detachable)),
				Val:               powerOffWithPowerKey,
			},
			{
				Name:              "ignore_short_power_key",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				Val:               ignoresShortPowerKey,
			},
			{
				Name:              "short_power_key",
				ExtraHardwareDeps: hwdep.D(hwdep.NoInternalDisplay()),
				Val:               powerOffWithShortPowerKey,
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
			s.Fatal("Failed ")
		}
	case powerOffWithPowerKey:
		// Perform test only if DUT is not a detachable.
		s.Log("Test power off with servo power key")
		if err := testRebootWithServoPowerKey(ctx, h); err != nil {
			s.Fatal("Failed to power off and power on with servo power key: ", err)
		}
	case ignoresShortPowerKey:
		// If DUT has internal display, expect 200ms power key press to be ignored.
		s.Log("Test that device with internal display ignores short power key press")
		if err := testIgnoreShortPowerKey(ctx, h); err != nil {
			s.Fatal("DUT unexpectedly shut down from short power key press: ", err)
		}
		s.Log("Test that device with internal display debounces very short power key press")
		if err := testPowerKeyDebounce(ctx, h); err != nil {
			s.Fatal("Expected power key pressed for 10ms to be debounced: ", err)
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

	testing.ContextLogf(ctx, "Sleep for %s ms", shortPowerKeyPressDur)
	if err := testing.Sleep(ctx, shortPowerKeyPressDur); err != nil {
		return errors.Wrapf(err, "failed to sleep for %s ms", shortPowerKeyPressDur)
	}

	testing.ContextLog(ctx, "Checking for S5 or G3 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S5", "G3"); err != nil {
		return errors.Wrap(err, "failed to get S5 or G3 powerstate")
	}

	testing.ContextLog(ctx, "Pressing power key")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
		return errors.Wrap(err, "failed to press power key on DUT")
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	return h.WaitConnect(ctx)
}

func testPowerKeyDebounce(ctx context.Context, h *firmware.Helper) error {
	readKeyPress := func(scanner *bufio.Scanner, keyCode string) error {

		regex := `Event.*time.*code\s(\d*)\s\(` + keyCode + `\)`
		expMatch := regexp.MustCompile(regex)

		text := make(chan string)
		go func() {
			for scanner.Scan() {
				text <- scanner.Text()
			}
			close(text)
		}()
		for {
			select {
			case <-time.After(2 * time.Second):
				return errors.New("did not detect keycode within expected time")
			case out := <-text:
				if match := expMatch.FindStringSubmatch(out); match != nil {
					testing.ContextLog(ctx, "key pressed: ", match)
					return nil
				}
			}
		}
	}

	if err := h.RequireRPCUtils(ctx); err != nil {
		return errors.Wrap(err, "requiring RPC utils")
	}
	testing.ContextLog(ctx, "Look for physical keyboard")
	res, err := h.RPCUtils.FindPhysicalKeyboard(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "during FindPhysicalKeyboard")
	}
	device := res.Path
	testing.ContextLog(ctx, "Device path: ", device)
	cmd := h.DUT.Conn().CommandContext(ctx, "evtest", device)
	stdout, err := cmd.StdoutPipe()
	scanner := bufio.NewScanner(stdout)
	cmd.Start()

	testing.ContextLog(ctx, "Pressing power key for 10ms")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(10*time.Millisecond)); err != nil {
		return errors.Wrap(err, "failed to press power key on DUT")
	}

	testing.ContextLog(ctx, "Read for power key press")
	if err := readKeyPress(scanner, "KEY_POWER"); err != nil {
		return errors.Wrap(err, "failed to read key")
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

	testing.ContextLog(ctx, "Expect S0 powerstate")
	currPowerState, err := h.Servo.GetECSystemPowerState(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get current power state")
	} else if currPowerState != "S0" {
		return errors.Errorf("Current power state is: %s, expected S0", currPowerState)
	}

	testing.ContextLog(ctx, "Get new boot id, compare to old")
	newBootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get current boot id")
	} else if newBootID != bootID {
		return errors.Errorf("boot ID unexpectedly changed from %s to %s", bootID, newBootID)
	}
	return nil
}

func testRebootWithServoPowerKey(ctx context.Context, h *firmware.Helper) error {
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "creating mode switcher")
	}

	if err := h.RequireConfig(ctx); err != nil {
		return errors.Wrap(err, "failed to create firmware config")
	}

	testing.ContextLog(ctx, "Rebooting to rec mode")
	if err = ms.RebootToMode(ctx, fwCommon.BootModeRecovery); err != nil {
		return errors.Wrap(err, "error during rebooting to rec mode")
	}

	if err := testing.Sleep(ctx, h.Config.FirmwareScreen); err != nil {
		return errors.Wrapf(err, "failed to sleep for %s seconds", h.Config.FirmwareScreen)
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	testing.ContextLog(ctx, "Pressing power key")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
		return errors.Wrap(err, "failed to press power key on DUT")
	}

	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep for 3 seconds")
	}

	testing.ContextLog(ctx, "Checking for S5 or G3 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S5", "G3"); err != nil {
		return errors.Wrap(err, "failed to get S5 or G3 powerstate")
	}

	testing.ContextLog(ctx, "Pressing power key")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
		return errors.Wrap(err, "failed to press power key on DUT")
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	return h.WaitConnect(ctx)
}

func testPowerdPowerOff(ctx context.Context, h *firmware.Helper) error {
	shutdownAndWake := func(shutDownDur time.Duration, expStates ...string) error {
		testing.ContextLog(ctx, "Pressing power key")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(shutDownDur)); err != nil {
			return errors.Wrap(err, "failed to press power key on DUT")
		}

		if err := testing.Sleep(ctx, 3*time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep for 3 seconds")
		}

		testing.ContextLogf(ctx, "Checking for %v powerstates", expStates)
		if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, expStates...); err != nil {
			return errors.Wrapf(err, "failed to get %v powerstates", expStates)
		}

		testing.ContextLog(ctx, "Send cmd to EC to wake up from deepsleep")
		h.Servo.RunECCommand(ctx, "help")

		testing.ContextLog(ctx, "Pressing power key")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress); err != nil {
			return errors.Wrap(err, "failed to press power key on DUT")
		}

		testing.ContextLog(ctx, "Waiting for S0 powerstate")
		if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
			return errors.Wrap(err, "failed to get S0 powerstate")
		}

		testing.ContextLog(ctx, "Sleep for 1 second")
		if err := testing.Sleep(ctx, 1*time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep for 1 second")
		}

		ms, err := firmware.NewModeSwitcher(ctx, h)
		if err != nil {
			return errors.Wrap(err, "creating mode switcher")
		}

		testing.ContextLog(ctx, "Perform mode aware reboot")
		return ms.ModeAwareReboot(ctx, firmware.ColdReset)
	}

	powerdDur := h.Config.HoldPwrButtonPowerOff
	noPowerdDur := h.Config.HoldPwrButtonNoPowerdShutdown

	if err := shutdownAndWake(powerdDur, "S5", "G3"); err != nil {
		return errors.Wrap(err, "failed shut down and wake with powerd")
	}

	testing.ContextLog(ctx, "stopping powerd")
	if _, err := h.DUT.Conn().CommandContext(ctx, "stop", "powerd").Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to stop powerd")
	}

	if err := shutdownAndWake(noPowerdDur, "G3"); err != nil {
		return errors.Wrap(err, "failed shut down and wake with no powerd")
	}

	if err := shutdownAndWake(powerdDur, "G3"); err != nil {
		return errors.Wrap(err, "failed shut down and wake with powerd")
	}

	testing.ContextLog(ctx, "stopping powerd")
	if _, err := h.DUT.Conn().CommandContext(ctx, "stop", "powerd").Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to stop powerd")
	}

	if err := shutdownAndWake(noPowerdDur, "S5", "G3"); err != nil {
		return errors.Wrap(err, "failed shut down and wake with no powerd")
	}

	return nil
}

func testRebootWithSettingPowerState(ctx context.Context, h *firmware.Helper) error {
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateOff); err != nil {
		return errors.Wrap(err, "failed to set 'power_state' to 'off'")
	}

	testing.ContextLog(ctx, "Pressing power key")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
		return errors.Wrap(err, "failed to press power key on DUT")
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	return h.WaitConnect(ctx)
}
