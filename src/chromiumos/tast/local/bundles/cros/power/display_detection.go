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
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type displayTestParams struct {
	tabletMode    bool
	displayInfoRe map[string]*regexp.Regexp
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisplayDetection,
		Desc:         "Verifies external display detection",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			"power.iterations",
			"power.chameleon_addr",
			"power.chameleon_display_port",
		},
		// To skip on duffy(Chromebox) with no internal display.
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Params: []testing.Param{{
			Name:    "dp_clamshell_mode",
			Fixture: "chromeLoggedIn",
			Val: displayTestParams{
				tabletMode: false,
				displayInfoRe: map[string]*regexp.Regexp{
					"connectorInfoPtrns": regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[DP]+.*`),
					"connectedPtrns":     regexp.MustCompile(`\[CONNECTOR:\d+:DP.*status: connected`),
					"modesPtrns":         regexp.MustCompile(`modes:\n.*"1920x1080":.60`),
				},
			},
			Timeout: 10 * time.Minute,
		}, {
			Name:    "dp_tablet_mode",
			Fixture: "chromeLoggedIn",
			Val: displayTestParams{
				tabletMode: true,
				displayInfoRe: map[string]*regexp.Regexp{
					"connectorInfoPtrns": regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[DP]+.*`),
					"connectedPtrns":     regexp.MustCompile(`\[CONNECTOR:\d+:DP.*status: connected`),
					"modesPtrns":         regexp.MustCompile(`modes:\n.*"1920x1080":.60`),
				},
			},
			Timeout: 10 * time.Minute,
		}, {
			Name:    "hdmi_clamshell_mode",
			Fixture: "chromeLoggedIn",
			Val: displayTestParams{
				tabletMode: false,
				displayInfoRe: map[string]*regexp.Regexp{
					"connectorInfoPtrns": regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[HDMI]+.*`),
					"connectedPtrns":     regexp.MustCompile(`\[CONNECTOR:\d+:HDMI.*status: connected`),
					"modesPtrns":         regexp.MustCompile(`modes:\n.*"1920x1080":.60`),
				},
			},
			Timeout: 10 * time.Minute,
		}, {
			Name:    "hdmi_tablet_mode",
			Fixture: "chromeLoggedIn",
			Val: displayTestParams{
				tabletMode: true,
				displayInfoRe: map[string]*regexp.Regexp{
					"connectorInfoPtrns": regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[HDMI]+.*`),
					"connectedPtrns":     regexp.MustCompile(`\[CONNECTOR:\d+:HDMI.*status: connected`),
					"modesPtrns":         regexp.MustCompile(`modes:\n.*"1920x1080":.60`),
				},
			},
			Timeout: 10 * time.Minute,
		}},
	})
}

func DisplayDetection(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	testOpt := s.Param().(displayTestParams)

	iterCount := 2 // Use default iteration 2 to plug-unplug external display.
	if iter, ok := s.Var("power.iterations"); ok {
		if iterCount, err = strconv.Atoi(iter); err != nil {
			s.Fatalf("Failed to parse iteration value %q: %v", iter, err)
		}
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if testOpt.tabletMode {
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
		if err != nil {
			s.Fatal("Failed to enable tablet mode: ", err)
		}
		defer cleanup(cleanupCtx)
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
		if portID, err = strconv.Atoi(port); err != nil {
			s.Fatalf("Failed to parse chameleon display port %q: %v", port, err)
		}
	}
	dp, err := che.NewPort(ctx, portID)
	if err != nil {
		s.Fatalf("Failed to create chameleon port %d: %v", portID, err)
	}
	// Cleanup
	defer dp.Unplug(cleanupCtx)

	for i := 1; i <= iterCount; i++ {
		s.Logf("Iteration: %d/%d", i, iterCount)
		s.Log("Plugging external display to DUT")
		if err := dp.Plug(ctx); err != nil {
			s.Fatal("Failed to plug chameleon port: ", err)
		}
		s.Log("chameleon plugged successfully")

		// Wait for DUT to detect external display.
		if err := dp.WaitVideoInputStable(ctx, 10*time.Second); err != nil {
			s.Fatal("Failed to wait for video input on chameleon board: ", err)
		}

		displayInfoPatterns := []*regexp.Regexp{
			testOpt.displayInfoRe["connectorInfoPtrns"],
			testOpt.displayInfoRe["connectedPtrns"],
			testOpt.displayInfoRe["modesPtrns"],
		}
		if err := waitForExternalMonitorCount(ctx, 1, displayInfoPatterns); err != nil {
			s.Fatal("Failed connecting external display: ", err)
		}

		if err := dp.Unplug(ctx); err != nil {
			s.Fatal("Failed to unplug chameleon port: ", err)
		}
		if err := waitForExternalMonitorCount(ctx, 0, nil); err != nil {
			s.Fatal("Failed unplugging external display: ", err)
		}
		s.Log("chameleon unplugged successfully")
	}
}

// waitForExternalMonitorCount verifies for the connected numberOfDisplays.
func waitForExternalMonitorCount(ctx context.Context, numberOfDisplays int, regexpPatterns []*regexp.Regexp) error {
	const DisplayInfoFile = "/sys/kernel/debug/dri/0/i915_display_info"
	// This regexp will skip pipe A since that's the internal display detection.
	displayInfo := regexp.MustCompile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "cat", DisplayInfoFile).Output()

		if err != nil {
			return errors.Wrap(err, "failed to run display info command ")
		}
		matchedString := displayInfo.FindAllString(string(out), -1)
		if len(matchedString) != numberOfDisplays {
			return errors.New("connected external display info not found")
		}
		if regexpPatterns != nil {
			for _, pattern := range regexpPatterns {
				if !pattern.MatchString(string(out)) {
					return errors.Errorf("failed %q error message", pattern)
				}
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
