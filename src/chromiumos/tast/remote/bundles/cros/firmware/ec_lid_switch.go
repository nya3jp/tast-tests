// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ECLidSwitch,
		Desc:     "Test EC Lid Switch",
		Contacts: []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:     []string{"group:firmware", "firmware_unstable"},
		Fixture:  fixture.NormalMode,
		// Skip on form factors without a lid.
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Lid()),
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
	})
}

// time constants
const (
	lidDelay       time.Duration = 2 * time.Second
	wakeDelay      time.Duration = 10 * time.Second
	noDelay        time.Duration = 0 * time.Second // minimal delay to make sure event is registered first
	readKeyDelay   time.Duration = 2 * time.Second
	readKeyTimeout time.Duration = 1 * time.Second
)

// regular expressions
var (
	reKeyPress = regexp.MustCompile(`Event.*time.*code\s\d*\s\((KEY\S+)\)`)
)

type lidSwitchArgs struct {
	ctx context.Context
	h   *firmware.Helper
	ms  *firmware.ModeSwitcher
}

func ECLidSwitch(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	if err := h.RequireRPCUtils(ctx); err != nil {
		s.Fatal("Requiring RPC utils: ", err)
	}
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create mode switcher: ", err)
	}

	args := &lidSwitchArgs{
		ctx: ctx,
		h:   h,
		ms:  ms,
	}

	s.Log("Check for errant keypresses on lid open/close")
	if err := checkKeyPress(args); err != nil {
		s.Fatal("Error checking key presses: ", err)
	}

	s.Log("Power off DUT and wake immediately")
	if err := powerOffAndWake(args, noDelay); err != nil {
		s.Fatal("Failed to poweroff and wake after delay: ", err)
	}

	s.Log("Power off DUT and wake after delay")
	if err := powerOffAndWake(args, wakeDelay); err != nil {
		s.Fatal("Failed to poweroff and wake after delay: ", err)
	}

	s.Log("Close DUT lid and wake immediately")
	if err := closeLidAndWake(args); err != nil {
		s.Fatal("Failed to close lid and wake immediately: ", err)
	}

	s.Log("Suspend DUT and wake immediately")
	if err := suspendAndWake(args, noDelay); err != nil {
		s.Fatal("Failed to suspend DUT and wake immediately: ", err)
	}

	s.Log("Suspend DUT and wake after delay")
	if err := suspendAndWake(args, wakeDelay); err != nil {
		s.Fatal("Failed to suspend DUT and wake after delay: ", err)
	}
}

func suspendAndWake(args *lidSwitchArgs, delay time.Duration) error {
	defer args.h.WaitConnect(args.ctx)
	testing.ContextLog(args.ctx, "Suspending DUT")
	cmd := args.h.DUT.Conn().CommandContext(args.ctx, "powerd_dbus_suspend")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to suspend DUT")
	}
	testing.ContextLog(args.ctx, "Checking for S0ix or S3 powerstate")
	if err := args.h.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0ix", "S3"); err != nil {
		return errors.Wrap(err, "failed to get S0ix or S3 powerstate")
	}

	// Used by main function to either immediately wake or wake after some delay.
	if err := testing.Sleep(args.ctx, delay); err != nil {
		return err
	}

	if err := args.h.Servo.CloseLid(args.ctx); err != nil {
		return err
	}
	// Delay by `lidDelay` to ensure lid is detected as closed.
	if err := testing.Sleep(args.ctx, lidDelay); err != nil {
		return err
	}
	if err := args.h.Servo.OpenLid(args.ctx); err != nil {
		return err
	}
	testing.ContextLog(args.ctx, "Waiting for S0 powerstate")
	err := args.h.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0")
	if err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}
	return nil
}

func closeLidAndWake(args *lidSwitchArgs) error {
	defer args.h.WaitConnect(args.ctx)
	if err := args.h.Servo.CloseLid(args.ctx); err != nil {
		return err
	}
	testing.ContextLog(args.ctx, "Check for G3 or S5 powerstate")
	err := args.h.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5")
	if err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}
	// Delay by `lidDelay` to ensure lid is detected as closed.
	if err := testing.Sleep(args.ctx, lidDelay); err != nil {
		return err
	}
	if err := args.h.Servo.OpenLid(args.ctx); err != nil {
		return err
	}
	testing.ContextLog(args.ctx, "Waiting for S0 powerstate")
	err = args.h.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0")
	if err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}
	return nil
}

func powerOffAndWake(args *lidSwitchArgs, delay time.Duration) error {
	defer args.h.WaitConnect(args.ctx)
	if err := args.ms.PowerOff(args.ctx); err != nil {
		return err
	}
	// The ms.PowerOff method checks for G3 or S5 but might just wait for DUT to be unreachable.
	// So it checks powerstate to make sure it reaches one of those two desired states.
	testing.ContextLog(args.ctx, "Check for G3 or S5 powerstate")
	err := args.h.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5")
	if err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}

	// Used by main function to either immediately wake or wake after some delay.
	if err := testing.Sleep(args.ctx, delay); err != nil {
		return err
	}

	if err := args.h.Servo.CloseLid(args.ctx); err != nil {
		return err
	}
	// Delay by `lidDelay` to ensure lid is detected as closed.
	if err := testing.Sleep(args.ctx, lidDelay); err != nil {
		return err
	}
	if err := args.h.Servo.OpenLid(args.ctx); err != nil {
		return err
	}
	testing.ContextLog(args.ctx, "Waiting for S0 powerstate")
	err = args.h.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0")
	if err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}
	return nil
}

func readKeyPresses(ctx context.Context, scanner *bufio.Scanner) error {
	text := make(chan string)
	go func() {
		for scanner.Scan() {
			text <- scanner.Text()
		}
		close(text)
	}()
	for {
		select {
		case <-time.After(readKeyTimeout):
			return nil
		case out := <-text:
			if match := reKeyPress.FindStringSubmatch(out); match != nil {
				return errors.Errorf("unexpected key pressed detected: %v", match)
			}
		}
	}
}

func checkKeyPress(args *lidSwitchArgs) error {
	// Skip keypress test if no keyboard is available on DUT.
	res, err := args.h.RPCUtils.FindPhysicalKeyboard(args.ctx, &empty.Empty{})
	if err != nil {
		testing.ContextLog(args.ctx, "No keyboard found, skipping keyboard tests: ", err)
		return nil
	}
	device := res.Path
	testing.ContextLogf(args.ctx, "Keyboard found at %v, checking for unexpected keypresses", device)

	if err := stopPowerd(args); err != nil {
		return errors.Wrap(err, "failed to stop powerd")
	}
	defer startPowerd(args)

	cmd := args.h.DUT.Conn().CommandContext(args.ctx, "evtest", device)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to pipe stdout from 'evtest' cmd")
	}
	scanner := bufio.NewScanner(stdout)
	cmd.Start()
	testing.ContextLog(args.ctx, "Started piping output from 'evtest'")
	defer cmd.Abort()

	if err := args.h.Servo.OpenLid(args.ctx); err != nil {
		return errors.Wrap(err, "error in checkKeyPress")
	}
	// Delay by `lidDelay` to ensure lid is detected as closed.
	if err := testing.Sleep(args.ctx, lidDelay); err != nil {
		return err
	}
	if err := args.h.Servo.CloseLid(args.ctx); err != nil {
		return errors.Wrap(err, "error in checkKeyPress")
	}
	// Delay reading by `readKeyDelay` to ensure any keypresses get logged to stdout.
	if err := testing.Sleep(args.ctx, readKeyDelay); err != nil {
		return err
	}
	if err := readKeyPresses(args.ctx, scanner); err != nil {
		return errors.Wrap(err, "error in checkKeyPress")
	}
	if err := args.h.Servo.OpenLid(args.ctx); err != nil {
		return errors.Wrap(err, "error in checkKeyPress")
	}

	return nil
}

func startPowerd(args *lidSwitchArgs) error {
	testing.ContextLog(args.ctx, "Starting powerd")
	startPowerd := args.h.DUT.Conn().CommandContext(args.ctx, "start", "powerd")
	return startPowerd.Run()
}

func stopPowerd(args *lidSwitchArgs) error {
	testing.ContextLog(args.ctx, "Stopping powerd")
	stopPowerd := args.h.DUT.Conn().CommandContext(args.ctx, "stop", "powerd")
	return stopPowerd.Run()
}
