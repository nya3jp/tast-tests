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
	readKeyTimeout time.Duration = 1 * time.Second
)

// regular expression
var (
	reKeyPress = regexp.MustCompile(`Event.*time.*code\s\d*\s\((KEY\S+)\)`)
)

type lidSwitchArgs struct {
	ctx context.Context
	h   *firmware.Helper
	ms  *firmware.ModeSwitcher
	ec  *firmware.ECTool
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
	ec := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain)

	args := &lidSwitchArgs{
		ctx: ctx,
		h:   h,
		ms:  ms,
		ec:  ec,
	}

	s.Log("check for errant keypresses and backlight state on lid open/close")
	res, err := h.RPCUtils.FindPhysicalKeyboard(ctx, &empty.Empty{})
	if err != nil {
		s.Log("No keyboard found, skipping keyboard tests: ", err)
	} else {
		if err := checkKeyPressAndBacklight(args, res.Path); err != nil {
			s.Fatal("Failed checking keypresses or backlight: ", err)
		}
	}
	s.Log("power off DUT and wake after delay")
	if err := powerOffAndWake(args, powerOffDUT, delayedWakeByLidSwitch); err != nil {
		s.Fatal("Failed to poweroff and wake after delay: ", err)
	}
	s.Log("close DUT lid and wake immediately")
	if err := powerOffAndWake(args, closeLid, wakeByLidSwitch); err != nil {
		s.Fatal("Failed to close lid and wake immediately: ", err)
	}
}

func delayedFunction(delay time.Duration, args *lidSwitchArgs, fn func(*lidSwitchArgs) error) error {
	testing.ContextLog(args.ctx, "Delaying function ")
	testing.Sleep(args.ctx, delay)
	return fn(args)
}

func closeLid(args *lidSwitchArgs) error {
	testing.ContextLog(args.ctx, "Closing DUT lid")
	return args.h.Servo.CloseLid(args.ctx)
}

func openLid(args *lidSwitchArgs) error {
	testing.ContextLog(args.ctx, "Opening DUT lid")
	return args.h.Servo.OpenLid(args.ctx)
}

func wakeByLidSwitch(args *lidSwitchArgs) error {
	if err := closeLid(args); err != nil {
		return err
	}
	testing.Sleep(args.ctx, lidDelay)
	if err := openLid(args); err != nil {
		return err
	}
	return nil
}

func delayedLidOpen(args *lidSwitchArgs) error {
	return delayedFunction(rpcDelay, args, openLid)
}

func delayedLidClose(args *lidSwitchArgs) error {
	return delayedFunction(rpcDelay, args, closeLid)
}

func delayedWakeByLidSwitch(args *lidSwitchArgs) error {
	return delayedFunction(wakeDelay, args, wakeByLidSwitch)
}

func powerOffDUT(args *lidSwitchArgs) error {
	return args.ms.PowerOff(args.ctx)
}

func powerOffAndWake(args *lidSwitchArgs, powerOffFunc, wakeFunc func(*lidSwitchArgs) error) error {
	if err := powerOffFunc(args); err != nil {
		return err
	}
	if err := wakeFunc(args); err != nil {
		return err
	}
	testing.ContextLog(args.ctx, "Waiting for S0 powerstate")
	if err := args.ms.WaitForPowerState(args.ctx, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}
	return nil
}

func hasKeyboardBacklight(args *lidSwitchArgs) bool {
	return args.ec.HasKBBacklight(args.ctx)
}

func getKeyboardBacklight(args *lidSwitchArgs) (int, error) {
	return args.ec.GetKBBacklight(args.ctx)
}

func setKeyboardBacklight(args *lidSwitchArgs, percent int) error {
	return args.ec.SetKBBacklight(args.ctx, percent)
}

func checkBacklight(args *lidSwitchArgs) error {
	currVal, err := getKeyboardBacklight(args)
	testing.ContextLog(args.ctx, "Current keyboard backlight value: ", currVal)
	if err != nil {
		return errors.Wrap(err, "error in getKeyboardBacklight")
	}
	if err = setKeyboardBacklight(args, 100); err != nil {
		return errors.Wrap(err, "error in setKeyboardBacklight")
	}
	if err = closeLid(args); err != nil {
		return errors.Wrap(err, "error in closeLid")
	}
	newVal, err := getKeyboardBacklight(args)
	if err != nil {
		return errors.Wrap(err, "error in getKeyboardBacklight")
	} else if newVal != 0 {
		return errors.New("keyboard backlight not disabled after lid close")
	}
	if err = openLid(args); err != nil {
		return errors.Wrap(err, "error in openLid")
	}
	newVal, err = getKeyboardBacklight(args)
	if err != nil {
		return errors.Wrap(err, "error in getKeyboardBacklight")
	} else if newVal == 0 {
		return errors.New("keyboard backlight unexpectedly remains disabled after lid open")
	}
	if err := setKeyboardBacklight(args, currVal); err != nil {
		return errors.Wrap(err, "error in setKeyboardBacklight")
	}
	return nil
}

func readKeyPresses(args *lidSwitchArgs, device string) error {
	testing.ContextLog(args.ctx, "Checking for unexpected keypresses")
	cmd := args.h.DUT.Conn().CommandContext(args.ctx, "evtest", device)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to pipe stdout from 'evtest' cmd")
	}
	scanner := bufio.NewScanner(stdout)
	cmd.Start()
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

func checkKeyPress(args *lidSwitchArgs, device string) error {
	if err := openLid(args); err != nil {
		return errors.Wrap(err, "error in checkKeyPress")
	}
	if err := readKeyPresses(args, device); err != nil {
		return errors.Wrap(err, "error in checkKeyPress")
	}
	if err := closeLid(args); err != nil {
		return errors.Wrap(err, "error in checkKeyPress")
	}
	if err := readKeyPresses(args, device); err != nil {
		return errors.Wrap(err, "error in checkKeyPress")
	}
	if err := openLid(args); err != nil {
		return errors.Wrap(err, "error in checkKeyPress")
	}
	return nil
}

func checkKeyPressAndBacklight(args *lidSwitchArgs, device string) error {
	if err := stopPowerd(args); err != nil {
		return errors.Wrap(err, "stopping powerd")
	}
	if err := checkKeyPress(args, device); err != nil {
		// restart powerd before raising error
		if err := startPowerd(args); err != nil {
			return err
		}
		return errors.Wrap(err, "error checking key presses")
	}
	if hasKeyboardBacklight(args) {
		if err := checkBacklight(args); err != nil {
			// restart powerd before raising error
			if err := startPowerd(args); err != nil {
				return err
			}
			return errors.Wrap(err, "error checking keyboard backlight")
		}
	} else {
		testing.ContextLog(args.ctx, "Keyboard doesn't support backlight, skipped backlight test")
	}
	if err := startPowerd(args); err != nil {
		return errors.Wrap(err, "starting powerd")
	}
	return nil
}

func startPowerd(args *lidSwitchArgs) error {
	startPowerd := args.h.DUT.Conn().CommandContext(args.ctx, "start", "powerd")
	return startPowerd.Run()
}

func stopPowerd(args *lidSwitchArgs) error {
	stopPowerd := args.h.DUT.Conn().CommandContext(args.ctx, "stop", "powerd")
	return stopPowerd.Run()
}
