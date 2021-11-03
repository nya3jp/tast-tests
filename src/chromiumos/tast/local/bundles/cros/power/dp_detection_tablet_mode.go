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

func init() {
	testing.AddTest(&testing.Test{
		Func:         DpDetectionTabletMode,
		Desc:         "Verifies DP detection via USB Type C port in tabletMode",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			"power.chameleon_addr",
			"power.chameleon_display_port",
		},
		// To skip on duffy(Chromebox) with no internal display.
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          chrome.LoggedIn(),
		Timeout:      8 * time.Minute,
	})
}

func DpDetectionTabletMode(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	var (
		connectorInfoPtrns = regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[DP]+.*`)
		connectedPtrns     = regexp.MustCompile(`\[CONNECTOR:\d+:DP.*status: connected`)
		modesPtrns         = regexp.MustCompile(`modes:\n.*"1920x1080":.60`)
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to enable tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

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

	const IterCount = 5
	for i := 1; i <= IterCount; i++ {
		s.Logf("Iteration: %d/%d", i, IterCount)
		s.Log("Plugging DP display to DUT")
		if err := dp.Plug(ctx); err != nil {
			s.Fatal("Failed to plug chameleon port: ", err)
		}
		s.Log("chameleon plugged successfully")

		// Wait for DUT to detect external display.
		if err := dp.WaitVideoInputStable(ctx, 10*time.Second); err != nil {
			s.Fatal("Failed to wait for video input on chameleon board: ", err)
		}

		displayInfoPatterns := []*regexp.Regexp{connectorInfoPtrns, connectedPtrns, modesPtrns}
		if err := waitForExternalMonitorCount(ctx, 1, displayInfoPatterns); err != nil {
			s.Fatal("Failed connecting external display: ", err)
		}

		s.Log("Unplugging DP display from DUT")
		if err := dp.Unplug(ctx); err != nil {
			s.Fatal("Failed to unplug chameleon port: ", err)
		}
		if err := waitForExternalMonitorCount(ctx, 0, nil); err != nil {
			s.Fatal("Failed unplugging external display: ", err)
		}
		s.Log("chameleon unplugged successfully")
	}
}

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
