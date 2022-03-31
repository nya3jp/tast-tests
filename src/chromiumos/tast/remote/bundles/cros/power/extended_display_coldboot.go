// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/chameleon"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type extendedDisplayTestParams struct {
	displayInfoRe     map[string]*regexp.Regexp
	checkTypeCAdapter bool
	ecStateToCheck    string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtendedDisplayColdboot,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies extended display functionality before and after performing cold boot",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome", "reboot"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.InternalDisplay()),
		Vars: []string{
			"servo",
			"power.chameleon_addr",
			"power.chameleon_display_port",
		},
		VarDeps: []string{"power.iteration"},
		Params: []testing.Param{{
			Name: "typec_dp",
			Val: extendedDisplayTestParams{
				displayInfoRe: map[string]*regexp.Regexp{
					"connectorInfoPtrns": regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[DP]+.*`),
					"connectedPtrns":     regexp.MustCompile(`\[CONNECTOR:\d+:DP.*status: connected`),
					"modesPtrns":         regexp.MustCompile(`modes:\n.*"1920x1080":.60`),
				},
				checkTypeCAdapter: true,
				ecStateToCheck:    "S5",
			},
			Timeout: 10 * time.Minute,
		}, {
			Name: "native_dp",
			Val: extendedDisplayTestParams{
				displayInfoRe: map[string]*regexp.Regexp{
					"connectorInfoPtrns": regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[DP]+.*`),
					"connectedPtrns":     regexp.MustCompile(`\[CONNECTOR:\d+:DP.*status: connected`),
					"modesPtrns":         regexp.MustCompile(`modes:\n.*"1920x1080":.60`),
				},
				ecStateToCheck: "S5",
			},
			Timeout: 10 * time.Minute,
		}, {
			Name: "typec_hdmi",
			Val: extendedDisplayTestParams{
				displayInfoRe: map[string]*regexp.Regexp{
					"connectorInfoPtrns": regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[HDMI]+.*`),
					"connectedPtrns":     regexp.MustCompile(`\[CONNECTOR:\d+:HDMI.*status: connected`),
					"modesPtrns":         regexp.MustCompile(`modes:\n.*"1920x1080":.60`),
				},
				checkTypeCAdapter: true,
				ecStateToCheck:    "G3",
			},
			Timeout: 10 * time.Minute,
		}, {
			Name: "native_hdmi",
			Val: extendedDisplayTestParams{
				displayInfoRe: map[string]*regexp.Regexp{
					"connectorInfoPtrns": regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[HDMI]+.*`),
					"connectedPtrns":     regexp.MustCompile(`\[CONNECTOR:\d+:HDMI.*status: connected`),
					"modesPtrns":         regexp.MustCompile(`modes:\n.*"1920x1080":.60`),
				},
				ecStateToCheck: "S5",
			},
			Timeout: 10 * time.Minute,
		}},
	})
}

func ExtendedDisplayColdboot(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	servoSpec, _ := s.Var("servo")
	dut := s.DUT()
	testOpt := s.Param().(extendedDisplayTestParams)

	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}
		}
	}(cleanupCtx)

	// Login to chrome and check for connected external display
	loginChrome := func(ctx context.Context) {
		// Perform a Chrome login.
		if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
			s.Fatal("Failed to login to Chrome: ", err)
		}

		if testOpt.checkTypeCAdapter {
			if err := checkTypeCAdapterDetection(ctx, dut); err != nil {
				s.Fatal("Failed to detect typec adapter for connected extended display: ", err)
			}
		}

		chameleonAddr := s.RequiredVar("power.chameleon_addr")

		// Use chameleon board as extended display. Make sure chameleon is connected.
		che, err := chameleon.New(ctx, chameleonAddr)
		if err != nil {
			s.Fatal("Failed to connect to chameleon board: ", err)
		}
		defer che.Close(cleanupCtx)

		portID := 3 // Use default port 3 for display.
		if port, ok := s.Var("power.chameleon_display_port"); ok {
			portID, err = strconv.Atoi(port)
			if err != nil {
				s.Fatalf("Failed to parse chameleon display port %q: %v", port, err)
			}
		}

		dp, err := che.NewPort(ctx, portID)
		if err != nil {
			s.Fatalf("Failed to create chameleon port %d: %v", portID, err)
		}

		if err := dp.Plug(ctx); err != nil {
			s.Fatal("Failed to plug chameleon port: ", err)
		}

		defer dp.Unplug(cleanupCtx)

		// Wait for DUT to detect external display.
		if err := dp.WaitVideoInputStable(ctx, 10*time.Second); err != nil {
			s.Fatal("Failed to wait for video input on chameleon board: ", err)
		}

		displayInfoPatterns := []*regexp.Regexp{
			testOpt.displayInfoRe["connectorInfoPtrns"],
			testOpt.displayInfoRe["connectedPtrns"],
			testOpt.displayInfoRe["modesPtrns"],
		}

		if err := externalDisplayDetection(ctx, dut, 1, displayInfoPatterns); err != nil {
			s.Fatal("Failed detecting external display: ", err)
		}
	}

	loginChrome(ctx)
	iter, err := strconv.Atoi(s.RequiredVar("power.iteration"))
	if err != nil {
		s.Fatal("Failed to convert string to integer: ", err)
	}
	for i := 1; i <= iter; i++ {
		s.Logf("Iteration: %d/%d ", i, iter)
		if err := powercontrol.ShutdownAndWaitForPowerState(ctx, pxy, dut, testOpt.ecStateToCheck); err != nil {
			s.Fatalf("Failed to shutdown and wait for %q powerstate: %v", testOpt.ecStateToCheck, err)
		}

		if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
			s.Fatal("Failed to power on DUT: ", err)
		}

		// Login chrome after waking from coldboot.
		loginChrome(ctx)

		// Perfoming prev_sleep_state check.
		const expectedPrevSleepState = 5
		if err := powercontrol.ValidatePrevSleepState(ctx, dut, expectedPrevSleepState); err != nil {
			s.Fatal("Failed to validate previous sleep state: ", err)
		}
	}
}

// externalDisplayDetection verifies connected extended display is detected or not.
func externalDisplayDetection(ctx context.Context, dut *dut.DUT, numberOfDisplays int, regexpPatterns []*regexp.Regexp) error {
	displayInfoFile := "/sys/kernel/debug/dri/0/i915_display_info"
	displayInfo := regexp.MustCompile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := dut.Conn().CommandContext(ctx, "cat", displayInfoFile).Output()
		if err != nil {
			return errors.Wrap(err, "failed to run display info command")
		}

		matchedString := displayInfo.FindAllString(string(out), -1)
		if len(matchedString) != numberOfDisplays {
			return errors.New("connected external display info not found")
		}

		for _, pattern := range regexpPatterns {
			if !(pattern).MatchString(string(out)) {
				return errors.Errorf("failed %q error message", pattern)
			}
		}

		return nil
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "please connect external display as required")
	}
	return nil
}

// checkTypeCAdapterDetection checks whether connected typec adapter is detected or not.
func checkTypeCAdapterDetection(ctx context.Context, dut *dut.DUT) error {
	usbDetectionRe := regexp.MustCompile(`If 0.*Class=.*5000M`)
	out, err := dut.Conn().CommandContext(ctx, "lsusb", "-t").Output()
	if err != nil {
		return errors.Wrap(err, "failed to execute lsusb command")
	}

	if !usbDetectionRe.MatchString(string(out)) {
		return errors.New("external display is not connected to DUT using typec adapter")
	}
	return nil
}
