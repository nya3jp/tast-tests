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
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DpSuspendResume,
		Desc:         "Verifies DP native port functionality with suspend-resume cycles",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			"power.chameleon_addr",
			"power.chameleon_display_port",
		},
		Fixture: "chromeLoggedIn",
		Timeout: 8 * time.Minute,
	})
}

func DpSuspendResume(ctx context.Context, s *testing.State) {
	var (
		C10PkgPattern         = regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
		connectorInfoPtrns    = regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[DP]+.*`)
		connectedPtrns        = regexp.MustCompile(`\[CONNECTOR:\d+:DP.*status: connected`)
		modesPtrns            = regexp.MustCompile(`modes:\n.*"1920x1080":.60`)
		SuspndFailurePattern  = regexp.MustCompile("Suspend failures: 0")
		FrmwreLogErrorPattern = regexp.MustCompile("Firmware log errors: 0")
		S0ixErrorPattern      = regexp.MustCompile("s0ix errors: 0")
	)
	const (
		SlpS0Cmd         = "cat /sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		PkgCstateCmd     = "cat /sys/kernel/debug/pmc_core/package_cstate_show"
		SuspendStressCmd = "suspend_stress_test -c 10"
	)
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()
	if chameleonAddr, ok := s.Var("power.chameleon_addr"); ok {
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
	}

	displayInfoPatterns := []*regexp.Regexp{connectorInfoPtrns, connectedPtrns, modesPtrns}
	if err := extDisplayDetection(ctx, 1, displayInfoPatterns); err != nil {
		s.Fatal("Failed plugging external display: ", err)
	}

	cmdOutput := func(cmd string) string {
		s.Logf("Executing command: %s", cmd)
		out, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output()
		if err != nil {
			s.Fatalf("Failed to execute %s command: %v", cmd, err)
		}
		return string(out)
	}
	slpOpSetPre := cmdOutput(SlpS0Cmd)
	pkgOpSetOutput := cmdOutput(PkgCstateCmd)
	matchSetPre := (C10PkgPattern).FindStringSubmatch(pkgOpSetOutput)
	if matchSetPre == nil {
		s.Fatal("Failed to match pre PkgCstate value: ", pkgOpSetOutput)
	}
	pkgOpSetPre := matchSetPre[1]
	stressOut := cmdOutput(SuspendStressCmd)

	suspendErrors := []*regexp.Regexp{SuspndFailurePattern, FrmwreLogErrorPattern, S0ixErrorPattern}
	for _, errmsg := range suspendErrors {
		if !errmsg.MatchString(string(stressOut)) {
			s.Fatalf("Expecting %q, but got failures %q", errmsg, string(stressOut))
		}
	}

	if err := extDisplayDetection(ctx, 1, displayInfoPatterns); err != nil {
		s.Fatal("Failed plugging external display after suspend-resume: ", err)
	}

	slpOpSetPost := cmdOutput(SlpS0Cmd)
	if slpOpSetPre == slpOpSetPost {
		s.Fatalf("SLP counter value %q must be different than the value noted most recently %q", slpOpSetPre, slpOpSetPost)
	}
	if slpOpSetPost == "0" {
		s.Fatal("SLP counter value must be non-zero, noted is: ", slpOpSetPost)
	}
	pkgOpSetPostOutput := cmdOutput(PkgCstateCmd)
	matchSetPost := (C10PkgPattern).FindStringSubmatch(pkgOpSetPostOutput)
	if matchSetPost == nil {
		s.Fatal("Failed to match post PkgCstate value: ", pkgOpSetPostOutput)
	}
	pkgOpSetPost := matchSetPost[1]
	if pkgOpSetPre == pkgOpSetPost {
		s.Fatalf("Package C10 value %q must be different than value noted most recently %q", pkgOpSetPre, pkgOpSetPost)
	}
	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		s.Fatal("Package C10 should be non-zero, but got: ", pkgOpSetPost)
	}
	if err := extDisplayDetection(ctx, 1, displayInfoPatterns); err != nil {
		s.Fatal("Failed to check plug status of external display: ", err)
	}
}

func extDisplayDetection(ctx context.Context, numberOfDisplays int, regexpPatterns []*regexp.Regexp) error {
	const DisplayInfoCommand = "cat /sys/kernel/debug/dri/0/i915_display_info"
	displCount, err := graphics.NumberOfOutputsConnected(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get connected displays ")
	}

	var DisplayInfo = regexp.MustCompile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)
	if displCount < 2 {
		return errors.New("external display is not connected")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "sh", "-c", DisplayInfoCommand).Output()

		if err != nil {
			return errors.Wrap(err, "failed to run display info command ")
		}
		matchedString := (DisplayInfo).FindAllString(string(out), -1)
		if len(matchedString) != numberOfDisplays {
			return errors.New("connected external display info not found")
		}
		if regexpPatterns != nil {
			for _, pattern := range regexpPatterns {
				if !(pattern).MatchString(string(out)) {
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
