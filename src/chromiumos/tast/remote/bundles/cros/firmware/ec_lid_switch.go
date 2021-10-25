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
		// Skip on form factors without a lid (chromebase, chromebox, chromebit, chromeslate)
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.SkipOnFormFactor(hwdep.Chromebase, hwdep.Chromebox, hwdep.Chromebit, hwdep.Chromeslate)),
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
	})
}

// time constants
const (
	lidDelay       time.Duration = 2 * time.Second
	rpcDelay       time.Duration = 2 * time.Second
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

	s.Log("check for errant keypresses on lid open/close")
	if err := checkKeyPress(args); err != nil {
		s.Fatal("Error checking key presses: ", err)
	}

	s.Log("check for unexpected keyboard backlight behaviour on lid open/close")
	if err := checkBacklight(args); err != nil {
		s.Fatal("Error checking keyboard backlight: ", err)
	}

	s.Log("power off DUT and wake immediately")
	if err := powerOffAndWake(args, noDelay); err != nil {
		s.Fatal("Failed to poweroff and wake after delay: ", err)
	}

	s.Log("power off DUT and wake after delay")
	if err := powerOffAndWake(args, wakeDelay); err != nil {
		s.Fatal("Failed to poweroff and wake after delay: ", err)
	}

	s.Log("close DUT lid and wake immediately")
	if err := closeLidAndWake(args); err != nil {
		s.Fatal("Failed to close lid and wake immediately: ", err)
	}
}

func closeLidAndWake(args *lidSwitchArgs) error {
	if err := args.h.Servo.CloseLid(args.ctx); err != nil {
		return err
	}
	testing.ContextLog(args.ctx, "Check for G3 or S5 powerstate")
	err := args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5")
	if err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}
	// delay waking by lidDelay at least lidDelay to ensure lid closed
	if err := testing.Sleep(args.ctx, lidDelay); err != nil {
		return err
	}
	if err := args.h.Servo.OpenLid(args.ctx); err != nil {
		return err
	}
	testing.ContextLog(args.ctx, "Waiting for S0 powerstate")
	err = args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0")
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
	// ms.PowerOff checks for G3 or S5 but might just wait for DUT to be unreachable; check powerstate to make sure
	testing.ContextLog(args.ctx, "Check for G3 or S5 powerstate")
	err := args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5")
	if err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}

	// used to either immediately wake or wake after some time
	if err := testing.Sleep(args.ctx, delay); err != nil {
		return err
	}

	if err := args.h.Servo.CloseLid(args.ctx); err != nil {
		return err
	}
	// delay waking by lidDelay at least lidDelay to ensure lid closed
	if err := testing.Sleep(args.ctx, lidDelay); err != nil {
		return err
	}
	if err := args.h.Servo.OpenLid(args.ctx); err != nil {
		return err
	}
	testing.ContextLog(args.ctx, "Waiting for S0 powerstate")
	err = args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0")
	if err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}
	return nil
}

func checkBacklight(args *lidSwitchArgs) error {
	// skip backlight test if keyboard does not support backlight feature
	if !args.h.Servo.HasKBBacklight(args.ctx) {
		testing.ContextLog(args.ctx, "Keyboard doesn't support backlight, skipped backlight test")
		return nil
	}

	if err := stopPowerd(args); err != nil {
		return errors.Wrap(err, "failed to stop powerd")
	}
	defer startPowerd(args)

	currVal, err := args.h.Servo.GetKBBacklight(args.ctx)
	testing.ContextLog(args.ctx, "Current keyboard backlight value: ", currVal)
	if err != nil {
		return errors.Wrap(err, "error in GetKBBacklight")
	}
	if err = args.h.Servo.SetKBBacklight(args.ctx, 100); err != nil {
		return errors.Wrap(err, "error in SetKBBacklight")
	}

	if err = args.h.Servo.CloseLid(args.ctx); err != nil {
		return errors.Wrap(err, "error in closeLid")
	}
	// delay by lidDelay at least lidDelay to ensure lid closed
	if err := testing.Sleep(args.ctx, lidDelay); err != nil {
		return err
	}
	newVal, err := args.h.Servo.GetKBBacklight(args.ctx)
	if err != nil {
		return errors.Wrap(err, "error in GetKBBacklight")
	} else if newVal != 0 {
		return errors.Errorf("keyboard backlight not disabled after lid close, value is %v", newVal)
	}
	if err = args.h.Servo.OpenLid(args.ctx); err != nil {
		return errors.Wrap(err, "error in openLid")
	}
	// delay by lidDelay at least lidDelay to ensure lid opened
	if err := testing.Sleep(args.ctx, lidDelay); err != nil {
		return err
	}

	// ERRROR SOURCE:
	newVal, err = args.h.Servo.GetKBBacklight(args.ctx)
	if err != nil {
		return errors.Wrap(err, "error in GetKBBacklight")
	} else if newVal == 0 {
		return errors.New("keyboard backlight unexpectedly remains disabled after lid open")
	}

	// reset backlight to original value
	if err := args.h.Servo.SetKBBacklight(args.ctx, currVal); err != nil {
		return errors.Wrap(err, "error in SetKBBacklight")
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
	return nil
}

func checkKeyPress(args *lidSwitchArgs) error {
	// skip keypress test if no keyboard is available
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
	// delay waking by lidDelay at least lidDelay to ensure lid closed
	if err := testing.Sleep(args.ctx, lidDelay); err != nil {
		return err
	}
	if err := args.h.Servo.CloseLid(args.ctx); err != nil {
		return errors.Wrap(err, "error in checkKeyPress")
	}
	// delay reading to make sure any keypresses get logged to stdout
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
