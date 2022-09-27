// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bufio"
	"context"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type lidSwitchTest int

const (
	checkKeyPresses lidSwitchTest = iota
	bootWithLid
	shutdownWithLid
	unsuspendWithLid
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECLidSwitch,
		Desc:         "Test EC Lid Switch",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Lid()),
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{
			{
				Name:              "check_key_press",
				Val:               checkKeyPresses,
				ExtraHardwareDeps: hwdep.D(hwdep.Keyboard()),
				ExtraAttr:         []string{"firmware_unstable"},
			},
			{
				Name: "open_lid_to_boot",
				Val:  bootWithLid,
				// Original test in suites: faft_ec, faft_ec_fw_qual, faft_ec_tot.
				ExtraAttr: []string{"firmware_ec"},
			},
			{
				Name:      "close_lid_to_shutdown",
				Val:       shutdownWithLid,
				ExtraAttr: []string{"firmware_ec"},
			},
			{
				// powerd_dbus_suspend is not very stable so leaving this in unstable.
				// This wasn't a test case in autotest so it's hard to determine the expected amount of stability.
				Name:      "open_lid_to_unsuspend",
				Val:       unsuspendWithLid,
				ExtraAttr: []string{"firmware_unstable"},
			},
		},
	})
}

const (
	lidDelay  time.Duration = 1 * time.Second
	wakeDelay time.Duration = 10 * time.Second
	noDelay   time.Duration = lidDelay // Just wait for lid state to change, no additional delay.
)

func ECLidSwitch(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	if err := h.RequireRPCUtils(ctx); err != nil {
		s.Fatal("Requiring RPC utils: ", err)
	}

	testMethod := s.Param().(lidSwitchTest)
	switch testMethod {
	case checkKeyPresses:
		s.Log("Check for errant keypresses on lid open/close")
		if err := checkKeyPressesWithLidClosed(ctx, h); err != nil {
			s.Fatal("Error checking key presses: ", err)
		}
	case bootWithLid:
		s.Log("Power off DUT and wake immediately")
		if err := bootWithLidOpen(ctx, h, noDelay); err != nil {
			s.Fatal("Failed to poweroff and wake immediately: ", err)
		}

		s.Log("Power off DUT and wake after delay")
		if err := bootWithLidOpen(ctx, h, wakeDelay); err != nil {
			s.Fatal("Failed to poweroff and wake after delay: ", err)
		}
	case shutdownWithLid:
		s.Log("Close DUT lid and wake immediately")
		if err := shutdownWithLidClose(ctx, h, noDelay); err != nil {
			s.Fatal("Failed to close lid and wake immediately: ", err)
		}

		s.Log("Close DUT lid and wake immediately")
		if err := shutdownWithLidClose(ctx, h, wakeDelay); err != nil {
			s.Fatal("Failed to close lid and wake immediately: ", err)
		}
	case unsuspendWithLid:
		s.Log("Suspend DUT and wake immediately")
		if err := suspendAndWakeWithLid(ctx, h, noDelay); err != nil {
			s.Fatal("Failed to suspend DUT and wake immediately: ", err)
		}

		s.Log("Suspend DUT and wake after delay")
		if err := suspendAndWakeWithLid(ctx, h, wakeDelay); err != nil {
			s.Fatal("Failed to suspend DUT and wake after delay: ", err)
		}
	}
}

func suspendAndWakeWithLid(ctx context.Context, h *firmware.Helper, delay time.Duration) error {
	testing.ContextLog(ctx, "Suspending DUT")
	cmd := h.DUT.Conn().CommandContext(ctx, "powerd_dbus_suspend", "--delay=5")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to suspend DUT")
	}

	testing.ContextLog(ctx, "Checking for S0ix or S3 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0ix", "S3"); err != nil {
		return errors.Wrap(err, "failed to get S0ix or S3 powerstate")
	}

	if err := h.Servo.CloseLid(ctx); err != nil {
		return err
	}

	// Used by main function to either immediately wake or wake after some delay.
	if err := testing.Sleep(ctx, delay); err != nil {
		return err
	}

	if err := h.Servo.OpenLid(ctx); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0")
	if err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}
	return nil
}

func shutdownWithLidClose(ctx context.Context, h *firmware.Helper, delay time.Duration) (reterr error) {
	// Log variables from powerd files to monitor unexpected settings.
	logCmd := `d="/var/lib/power_manager"; for f in $(ls -A $d); do echo "$f: $(cat $d/$f)"; done`
	out, err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", logCmd).Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to read files in /var/lib/power_manager")
	}
	testing.ContextLog(ctx, "Files in /var/lib/power_manager: ", string(out))

	if err := h.Servo.CloseLid(ctx); err != nil {
		return err
	}

	// This usually takes longer than usual to reach G3/S5, so increase timeout.
	testing.ContextLog(ctx, "Check for G3 or S5 powerstate")
	err = h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 2*firmware.PowerStateTimeout, "G3", "S5")
	if err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}

	// Delay by `lidDelay` to ensure lid is detected as closed.
	if err := testing.Sleep(ctx, delay); err != nil {
		return err
	}

	if err := h.Servo.OpenLid(ctx); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	err = h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0")
	if err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	if err := h.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to DUT")
	}
	return nil
}

func bootWithLidOpen(ctx context.Context, h *firmware.Helper, delay time.Duration) error {
	testing.ContextLog(ctx, "Shutdown dut")
	if err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", "(sleep 2; /sbin/shutdown -P now) &").Start(); err != nil {
		return errors.Wrap(err, "failed to run `/sbin/shutdown -P now` cmd")
	}

	testing.ContextLog(ctx, "Check for G3 or S5 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 2*firmware.PowerStateTimeout, "G3", "S5"); err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}

	if err := h.Servo.CloseLid(ctx); err != nil {
		return err
	}

	// Used by main function to either immediately wake or wake after some delay.
	testing.ContextLogf(ctx, "Delay opening lid by %s", delay)
	if err := testing.Sleep(ctx, delay); err != nil {
		return err
	}

	if err := h.Servo.OpenLid(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	if err := h.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to dut")
	}
	return nil
}

func checkKeyPressesWithLidClosed(ctx context.Context, h *firmware.Helper) (reterr error) {
	res, err := h.RPCUtils.FindPhysicalKeyboard(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	device := res.Path
	testing.ContextLogf(ctx, "Keyboard found at %v, checking for unexpected keypresses", device)

	powerdCmd := "mkdir -p /tmp/power_manager && " +
		"echo 0 > /tmp/power_manager/use_lid && " +
		"mount --bind /tmp/power_manager /var/lib/power_manager && " +
		"restart powerd"
	if err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", powerdCmd).Run(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to set use_lid")
	}
	defer func(ctx context.Context) {
		restartPowerd := "umount /var/lib/power_manager && restart powerd"
		if err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", restartPowerd).Run(ssh.DumpLogOnError); err != nil {
			reterr = errors.Wrap(err, "failed to restore powerd settings")
		}
	}(ctx)

	cmd := h.DUT.Conn().CommandContext(ctx, "evtest", device)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to pipe stdout from 'evtest' cmd")
	}
	scanner := bufio.NewScanner(stdout)
	cmd.Start()
	testing.ContextLog(ctx, "Started piping output from 'evtest'")
	defer cmd.Abort()

	readKeyPress := func() error {
		text := make(chan string)
		go func() {
			defer close(text)
			for scanner.Scan() {
				text <- scanner.Text()
			}
		}()
		for {
			select {
			case <-time.After(5 * time.Second):
				return nil
			case out := <-text:
				if match := regexp.MustCompile(`Event.*time.*code\s(\d*)\s\(\S+\)`).FindStringSubmatch(out); match != nil {
					return errors.Errorf("unexpected key pressed detected: %s", match[0])
				}
			}
		}
	}

	// Make sure lid is open in case DUT is in closed lid state at test start.
	if err := h.Servo.OpenLid(ctx); err != nil {
		return errors.Wrap(err, "error opening lid")
	}

	// Delay by `lidDelay` to ensure lid is detected as open before re-closing.
	if err := testing.Sleep(ctx, lidDelay); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "error closing lid")
	}

	testing.ContextLog(ctx, "Checking for unexpected keypresses on lid close")
	if err := readKeyPress(); err != nil {
		return errors.Wrap(err, "expected no keypresses with lid closed")
	}

	if err := h.Servo.OpenLid(ctx); err != nil {
		return errors.Wrap(err, "error opening lid")
	}

	testing.ContextLog(ctx, "Checking for unexpected keypresses on lid open")
	if err := readKeyPress(); err != nil {
		return errors.Wrap(err, "expected no keypresses with lid closed")
	}

	return nil
}
