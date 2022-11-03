// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WaysToSuspendDUT,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies ways to suspend ChromeOS device",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"servo"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Params: []testing.Param{{
			Name:    "clamshell",
			Val:     false,
			Timeout: 30 * time.Minute,
		}, {
			Name:    "tablet",
			Val:     true,
			Timeout: 30 * time.Minute,
		}},
	})
}

func WaysToSuspendDUT(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	dut := s.DUT()

	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	disableIdleSuspendFile := "/var/lib/power_manager/disable_idle_suspend"
	idleSuspendOut, err := linuxssh.ReadFile(ctx, dut.Conn(), disableIdleSuspendFile)
	if err != nil {
		s.Fatal("Failed to read disable_idle_suspend file: ", err)
	}

	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			if err := firmware.BootDutViaPowerPress(ctx, h, dut); err != nil {
				s.Error("Failed to power on DUT in cleanup: ", err)
			}
		}
		disableIdleSuspendCommand := fmt.Sprintf("echo %s > /var/lib/power_manager/disable_idle_suspend", strings.TrimSpace(string(idleSuspendOut)))
		if err := dut.Conn().CommandContext(ctx, "sh", "-c", disableIdleSuspendCommand).Run(); err != nil {
			s.Error("Failed to execute disableIdleSuspend command: ", err)
		}
		if err := dut.Conn().CommandContext(ctx, "restart", "powerd").Run(); err != nil {
			s.Error("Failed to run restart powerd command: ", err)
		}
	}(cleanupCtx)

	// waitConnectWithCtx waits for DUT to be connected within given duration.
	waitConnectWithCtx := func(ctx context.Context, duration time.Duration) {
		waitCtx, cancel := context.WithTimeout(ctx, duration)
		defer cancel()
		if err := dut.WaitConnect(waitCtx); err != nil {
			s.Fatal("Failed to wait connect after suspend_stress_test for 5 minutes: ", err)
		}
	}

	// waitUnreachableWithCtx waits for DUT to be unreachable.
	waitUnreachableWithCtx := func(ctx context.Context) {
		suspendCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err := dut.WaitUnreachable(suspendCtx); err != nil {
			s.Fatal("Failed to wait for unreachable: ", err)
		}
	}

	isTabletMode := s.Param().(bool)
	if isTabletMode {
		tabletModeOn := "tabletmode on"
		if _, err := h.Servo.CheckAndRunTabletModeCommand(ctx, tabletModeOn); err != nil {
			s.Fatal("Unablet to reset EC tablet mode setting: ", err)
		}
		tabletModeOff := "tabletmode off"
		defer h.Servo.CheckAndRunTabletModeCommand(cleanupCtx, tabletModeOff)
	}

	// Performs Chrome login.
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login Chrome: ", err)
	}

	initialBrightnessVal, err := systemBrightness(ctx, dut)
	if err != nil {
		s.Fatal("Failed to get system brightness: ", err)
	}
	defer dut.Conn().CommandContext(cleanupCtx, "backlight_tool", fmt.Sprintf("--set_brightness=%d", initialBrightnessVal)).Run()

	isACPlug := true
	if err := powercontrol.PlugUnplugCharger(ctx, h, isACPlug); err != nil {
		s.Fatal("Failed to plug charger: ", err)
	}

	s.Log("Waiting for system to enter screen dim state, idle state and to suspend DUT with AC charger plugged")
	if err := performSuspendViaDisplay(ctx, dut, initialBrightnessVal); err != nil {
		s.Fatal("Failed to perform DUT suspend via screen dim/screen off operation with AC charger plugged: ", err)
	}
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurShortPress); err != nil {
		s.Fatal("Failed to wake DUT via servo: ", err)
	}
	waitConnectWithCtx(ctx, 60*time.Second)

	if err := powercontrol.PlugUnplugCharger(ctx, h, !isACPlug); err != nil {
		s.Fatal("Failed to remove charger: ", err)
	}
	defer h.SetDUTPower(cleanupCtx, isACPlug)

	s.Log("Waiting for system to enter screen dim state, idle state and to suspend DUT with AC charger unplugged")
	if err := performSuspendViaDisplay(ctx, dut, initialBrightnessVal); err != nil {
		s.Fatal("Failed to perform DUT suspend via screen dim/screen off operation with AC charger unplugged: ", err)
	}
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurShortPress); err != nil {
		s.Fatal("Failed to wake DUT via servo: ", err)
	}
	waitConnectWithCtx(ctx, 60*time.Second)

	if err := h.Servo.CloseLid(ctx); err != nil {
		s.Fatal("Failed to close lid: ", err)
	}
	waitUnreachableWithCtx(ctx)

	if err := h.Servo.OpenLid(ctx); err != nil {
		s.Error("Failed to open lid: ", err)
	}
	waitConnectWithCtx(ctx, 60*time.Second)

	s.Log("Performing suspend with powerd_dbus_suspend command")
	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := dut.Conn().CommandContext(cmdCtx, "powerd_dbus_suspend").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		s.Fatal("Failed to execute powerd_dbus_suspend command: ", err)
	}
	waitUnreachableWithCtx(ctx)

	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurShortPress); err != nil {
		s.Fatal("Failed to press power button through servo: ", err)
	}
	waitConnectWithCtx(ctx, 60*time.Second)

	s.Log("Performing suspend with set_short_powerd_timeouts command")
	cmdCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := dut.Conn().CommandContext(cmdCtx, "set_short_powerd_timeouts").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		s.Fatal("Failed to execute set short powerd timeouts: ", err)
	}
	defer dut.Conn().CommandContext(cleanupCtx, "set_short_powerd_timeouts", "--reset").Run()

	ecPowerS0ixState := "S0ix"
	if err := h.WaitForPowerStates(ctx, 1*time.Second, 40*time.Second, ecPowerS0ixState); err != nil {
		s.Fatal("Failed to verify EC power state after set_short_powerd_timeouts: ", err)
	}
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurShortPress); err != nil {
		s.Fatal("Failed to press power button through servo: ", err)
	}
	waitConnectWithCtx(ctx, 60*time.Second)

	if err := dut.Conn().CommandContext(ctx, "set_short_powerd_timeouts", "--reset").Run(); err != nil {
		s.Fatal("Failed to reset short powerd timeouts: ", err)
	}

	// Performing suspend_stress_test with 1 cycle before executing actual
	// suspend_stress_test command to ensure that there are zero
	// Premature wakes, Suspend failures, firmware log errors, s0ix errors.
	if err := powercontrol.PerformSuspendStressTest(ctx, dut, 1); err != nil {
		s.Fatal("Failed to perform suspend stress test to check for zero errors: ", err)
	}

	s.Log("Performing suspend with suspend_stress_test command")
	minTime := "300"
	maxTime := "300"
	cmdCtx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := dut.Conn().CommandContext(cmdCtx,
		"suspend_stress_test", "-c", "1",
		"--suspend_min", minTime,
		"--suspend_max", maxTime).Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		s.Fatal("Failed to execute suspend_stress_test: ", err)
	}
	if err := powercontrol.WaitForSuspendState(ctx, h); err != nil {
		s.Fatal("Failed to verify suspend state after suspend_stress_test: ", err)
	}
	waitConnectWithCtx(ctx, 310*time.Second)
}

// systemBrightness returns system display brightness value.
func systemBrightness(ctx context.Context, dut *dut.DUT) (int, error) {
	bnsOut, err := dut.Conn().CommandContext(ctx, "backlight_tool", "--get_brightness").Output()
	if err != nil {
		return 0, errors.Wrap(err, "failed to execute backlight_tool command")
	}
	brightnessValue, err := strconv.Atoi(strings.TrimSpace(string(bnsOut)))
	if err != nil {
		return 0, errors.Wrap(err, "failed to convert string to integer")
	}
	return brightnessValue, nil
}

// waitForScreenDimState waits for system display to go to dim state.
func waitForScreenDimState(ctx context.Context, dut *dut.DUT, initialbrightnessVal int) error {
	testing.ContextLog(ctx, "Waiting for display to go dim")
	dim := false
	return testing.Poll(ctx, func(ctx context.Context) error {
		brightnessValue, err := systemBrightness(ctx, dut)
		if err != nil {
			return errors.Wrap(err, "failed to get system brightness")
		}
		if brightnessValue < initialbrightnessVal {
			testing.ContextLog(ctx, "System screen went to dim state")
			dim = true
		}
		if !dim {
			return errors.New("system display failed to go to dim state")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Minute})
}

// waitForSystemIdle waits for system display to go to idle state.
func waitForSystemIdle(ctx context.Context, dut *dut.DUT) error {
	testing.ContextLog(ctx, "Waiting for display to go off")
	idle := false
	return testing.Poll(ctx, func(ctx context.Context) error {
		brightnessValue, err := systemBrightness(ctx, dut)
		if err != nil {
			return errors.Wrap(err, "failed to get system brightness")
		}
		if brightnessValue == 0 {
			testing.ContextLog(ctx, "System went idle state")
			idle = true
		}
		if !idle {
			return errors.New("system display failed to go to idle state")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Minute})
}

// performSuspendViaDisplay performs DUT suspend by waiting for system display
// to enter into dim state and idle state.
func performSuspendViaDisplay(ctx context.Context, dut *dut.DUT, initialbrightnessVal int) error {
	enableIdleSuspendCommand := "echo 0 > /var/lib/power_manager/disable_idle_suspend"
	if err := dut.Conn().CommandContext(ctx, "sh", "-c", enableIdleSuspendCommand).Run(); err != nil {
		return errors.Wrap(err, "failed to execute enableIdleSuspend command")
	}
	if err := dut.Conn().CommandContext(ctx, "restart", "powerd").Run(); err != nil {
		return errors.Wrap(err, "failed to run restart powerd command")
	}
	if err := waitForScreenDimState(ctx, dut, initialbrightnessVal); err != nil {
		return errors.Wrap(err, "failed to wait for system screen to become dim")
	}
	if err := waitForSystemIdle(ctx, dut); err != nil {
		return errors.Wrap(err, "failed to wait for system to enter idle state")
	}
	// Once system display goes to idle state it is expected
	// for DUT to suspend.
	suspendCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := dut.WaitUnreachable(suspendCtx); err != nil {
		return errors.Wrap(err, "failed to wait for unreachable after system idle")
	}
	return nil
}
