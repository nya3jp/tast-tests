// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/chameleon"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
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
			if err := turnOnDut(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}
		}
	}(cleanupCtx)

	// Login to chrome and check for connected external display
	loginChrome := func(ctx context.Context) {
		// Perform a Chrome login.
		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}

		client := security.NewBootLockboxServiceClient(cl.Conn)
		if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to start Chrome: ", err)
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
	iter := 10
	for i := 1; i <= iter; i++ {
		s.Logf("Iteration: %d/%d ", i, iter)
		if err := performShutdown(ctx, dut); err != nil {
			s.Fatal("Failed to perform shutdown: ", err)
		}

		if err := verifyECState(ctx, pxy, testOpt.ecStateToCheck); err != nil {
			s.Fatalf("Failed to enter %s state: %v", testOpt.ecStateToCheck, err)
		}

		if err := turnOnDut(ctx, pxy, dut); err != nil {
			s.Fatal("Failed to power on dut after shutdown: ", err)
		}

		// Login chrome after waking from coldboot.
		loginChrome(ctx)

		// Perfoming prev_sleep_state check.
		const (
			prevSleepStateCmd      = "cbmem -c | grep 'prev_sleep_state' | tail -1"
			expectedPrevSleepState = 5
		)

		out, err := dut.Conn().CommandContext(ctx, "sh", "-c", prevSleepStateCmd).Output()
		if err != nil {
			s.Fatal("Failed to execute cbmem command: ", err)
		}

		actualPrevSleepState, err := strconv.Atoi(strings.Split(strings.Replace(string(out), "\n", "", -1), " ")[1])
		if err != nil {
			s.Fatal("String conversion failed: ", err)
		}

		if actualPrevSleepState != expectedPrevSleepState {
			s.Fatalf("Unexpected previous sleep state, want %q but got %q", expectedPrevSleepState, actualPrevSleepState)
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

// performShutdown performs cold boot with shutdown command.
func performShutdown(ctx context.Context, dut *dut.DUT) error {
	testing.ContextLog(ctx, "Performing coldboot")
	if err := dut.Conn().CommandContext(ctx, "shutdown", "-h", "now").Run(); err != nil {
		return errors.Wrap(err, "failed to execute shutdown command")
	}
	sdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		return errors.Wrap(err, "failed to wait for unreachable")
	}
	return nil
}

// turnOnDut performs power button normal press to wake up DUT.
func turnOnDut(ctx context.Context, pxy *servo.Proxy, dut *dut.DUT) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			return errors.Wrap(err, "failed to power normal press")
		}
		if err := dut.WaitConnect(waitCtx); err != nil {
			return errors.Wrap(err, "failed to wait connect DUT")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Minute}); err != nil {
		return err
	}
	return nil
}

// verifyECState performs validation of ecState check.
func verifyECState(ctx context.Context, pxy *servo.Proxy, ecState string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		pwrState, err := pxy.Servo().GetECSystemPowerState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get power state")
		}

		if pwrState != ecState {
			return errors.Errorf("unexpected EC power state, want %q state; got %q state", ecState, pwrState)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}
	return nil
}

// checkTypeCAdapterDetection checks whether connected typec adapter is detected or not.
func checkTypeCAdapterDetection(ctx context.Context, dut *dut.DUT) error {
	usbDetectionRe := regexp.MustCompile(`Class=.*(480M|5000M|10G|20G)`)
	out, err := dut.Conn().CommandContext(ctx, "lsusb", "-t").Output()
	if err != nil {
		return errors.Wrap(err, "failed to execute lsusb command")
	}

	if !usbDetectionRe.MatchString(string(out)) {
		return errors.New("external display is not connected to DUT using typec adapter")
	}
	return nil
}
