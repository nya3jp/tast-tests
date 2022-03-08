// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type extendedDisplayFunctionTestParams struct {
	powerMode              string
	ecStateToCheck         string
	expectedPrevSleepState int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtendedDisplayFunctionality,
		Desc:         "Verifies type-C extended display functionality before and after performing cold boot and warm boot",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome", "reboot"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.InternalDisplay()),
		Timeout:      5 * time.Minute,
		Vars: []string{
			"servo",
			"power.mode", // Optional. Expecting "tablet". By default power.mode will be "clamshell".
		},
		Params: []testing.Param{{
			Name: "typec_hdmi_shutdown",
			Val: extendedDisplayFunctionTestParams{
				powerMode:              "shutdown_command",
				ecStateToCheck:         "S5",
				expectedPrevSleepState: 5,
			},
		}, {
			Name: "typec_hdmi_reboot",
			Val: extendedDisplayFunctionTestParams{
				powerMode:              "reboot_command",
				expectedPrevSleepState: 0,
			},
		}, {
			Name: "typec_hdmi_powerbtn",
			Val: extendedDisplayFunctionTestParams{
				powerMode:              "powerbtn_shutdown",
				ecStateToCheck:         "S5",
				expectedPrevSleepState: 5,
			},
		}},
	})
}

func ExtendedDisplayFunctionality(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	servoSpec, _ := s.Var("servo")
	dut := s.DUT()
	testOpt := s.Param().(extendedDisplayFunctionTestParams)

	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	// Get the initial tablet_mode_angle settings to restore at the end of test.
	re := regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
	tabletOut, err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle").Output()
	if err != nil {
		s.Fatal("Failed to retrieve tablet_mode_angle settings: ", err)
	}
	m := re.FindSubmatch(tabletOut)
	if len(m) != 3 {
		s.Fatalf("Failed to get initial tablet_mode_angle settings: got submatches %+v", m)
	}
	initLidAngle := m[1]
	initHys := m[2]

	defaultMode := "clamshell"
	if mode, ok := s.Var("power.mode"); ok {
		defaultMode = mode
	}

	if defaultMode == "tablet" {
		// Set tabletModeAngle to 0 to force the DUT into tablet mode.
		testing.ContextLog(ctx, "Put DUT into tablet mode")
		if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", "0", "0").Run(); err != nil {
			s.Fatal("Failed to set DUT into tablet mode: ", err)
		}
	}

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}
		}
		if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", string(initLidAngle), string(initHys)).Run(); err != nil {
			s.Fatal("Failed to restore tablet_mode_angle to the original settings: ", err)
		}
	}(cleanupCtx)

	// Perform a Chrome login.
	s.Log("Login to Chrome")
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}

	// Verifying external display detection before shutdown/reboot.
	if err := externalTypecDisplayDetection(ctx, dut, 1); err != nil {
		s.Fatalf("Failed detecting external display before %q: %v", testOpt.powerMode, err)
	}

	if testOpt.powerMode == "shutdown_command" {
		testing.ContextLog(ctx, "Executing shutdown command")
		if err := powercontrol.ShutdownAndWaitForPowerState(ctx, pxy, dut, testOpt.ecStateToCheck); err != nil {
			s.Fatal("Failed to shutdown DUT and check EC state: ", err)
		}
	}

	if testOpt.powerMode == "reboot_command" {
		testing.ContextLog(ctx, "Rebooting the DUT")
		if err := performReboot(ctx, dut); err != nil {
			s.Fatal("Failed to perform DUT reboot: ", err)
		}
	}

	if testOpt.powerMode == "typec_hdmi_powerbtn" {
		testing.ContextLog(ctx, "Shutdown DUT with power button long press via servo")
		if err := shutdownWithPowerButtonViaServo(ctx, pxy, dut); err != nil {
			s.Fatal("Failed to shutdown DUT with power long press via servo: ", err)
		}
		if err := verifyECPowerState(ctx, pxy, testOpt.ecStateToCheck); err != nil {
			s.Fatal("Failed to check EC power state: ", err)
		}
	}

	if testOpt.ecStateToCheck != "" {
		// Performing power normal press to power ON DUT.
		if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
			s.Fatal("Failed to power on DUT: ", err)
		}
	}

	// Verifying external display detection after shutdown/reboot.
	if err := externalTypecDisplayDetection(ctx, dut, 1); err != nil {
		s.Fatalf("Failed detecting external display after %q: %v", testOpt.powerMode, err)
	}

	// Perfoming prev_sleep_state check.
	if err := powercontrol.ValidatePrevSleepState(ctx, dut, testOpt.expectedPrevSleepState); err != nil {
		s.Fatal("Failed to validate previous sleep state: ", err)
	}
}

// externalTypecDisplayDetection verifies extended display is connected through typec adapter or not.
func externalTypecDisplayDetection(ctx context.Context, dut *dut.DUT, numberOfDisplays int) error {
	// Checking whether typec adapter connected to DUT.
	lsbOut, err := dut.Conn().CommandContext(ctx, "lsusb", "-t").Output()
	if err != nil {
		return errors.Wrap(err, "failed to execute lsusb command")
	}

	usbDetectionRe := regexp.MustCompile(`If 0.*Class=.*5000M`)
	if !usbDetectionRe.MatchString(string(lsbOut)) {
		return errors.New("failed to detect typec adapter")
	}

	var (
		displayInfoRe     = regexp.MustCompile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)
		connectorInfoRe   = regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[DP]+.*`)
		connectedStatusRe = regexp.MustCompile(`\[CONNECTOR:\d+:DP.*status: connected`)
		modesRe           = regexp.MustCompile(`modes:\n.*"1920x1080":.60`)
	)

	displayInfoFile := "/sys/kernel/debug/dri/0/i915_display_info"
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := linuxssh.ReadFile(ctx, dut.Conn(), displayInfoFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read %q file", displayInfoFile)
		}

		matchedString := displayInfoRe.FindAllString(string(out), -1)
		if len(matchedString) != numberOfDisplays {
			return errors.New("connected external display info not found")
		}

		displayInfoPatterns := []*regexp.Regexp{connectorInfoRe, connectedStatusRe, modesRe}
		for _, pattern := range displayInfoPatterns {
			if !pattern.MatchString(string(out)) {
				return errors.Errorf("failed to find display info match %q", pattern)
			}
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 20 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "unable to find external display")
	}
	return nil
}

// shutdownWithPowerButtonViaServo performs shutdown DUT with power button long press via servo.
func shutdownWithPowerButtonViaServo(ctx context.Context, pxy *servo.Proxy, dut *dut.DUT) error {
	if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress); err != nil {
		return errors.Wrap(err, "failed to power long press")
	}
	sdCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		return errors.Wrap(err, "failed to wait DUT to be unreachable")
	}
	return nil
}

// verifyECPowerState verifies EC powerState of DUT.
func verifyECPowerState(ctx context.Context, pxy *servo.Proxy, powerState string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		got, err := pxy.Servo().GetECSystemPowerState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get EC power state")
		}
		if want := powerState; got != want {
			return errors.Errorf("unexpected DUT EC power state = got %q, want %q", got, want)
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second})
}

// performReboot performs DUT reboot.
func performReboot(ctx context.Context, dut *dut.DUT) error {
	if err := dut.Reboot(ctx); err != nil {
		return errors.Wrap(err, "failed to reboot the DUT")
	}
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := dut.WaitConnect(waitCtx); err != nil {
		return errors.Wrap(err, "failed to wait connect to DUT")
	}
	return nil
}
