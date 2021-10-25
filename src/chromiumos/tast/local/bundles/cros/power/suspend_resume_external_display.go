// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SuspendResumeExternalDisplay,
		Desc:         "Verifies suspend-resume with USB type-C display functionality check",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:typec"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Timeout:      5 * time.Minute,
	})
}

func SuspendResumeExternalDisplay(ctx context.Context, s *testing.State) {
	var (
		C10PkgPattern         = regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
		SuspndFailurePattern  = regexp.MustCompile("Suspend failures: 0")
		FrmwreLogErrorPattern = regexp.MustCompile("Firmware log errors: 0")
		S0ixErrorPattern      = regexp.MustCompile("s0ix errors: 0")
	)
	const (
		SlpS0Cmd         = "cat /sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		PkgCstateCmd     = "cat /sys/kernel/debug/pmc_core/package_cstate_show"
		SuspendStressCmd = "suspend_stress_test -c 10"
	)
	cmdOutput := func(cmd string) string {
		out, err := testexec.CommandContext(ctx, "bash", "-c", cmd).Output()
		if err != nil {
			s.Fatalf("Failed to execute %s command: %v", cmd, err)
		}
		return string(out)
	}
	if err := externalDisplayDetection(ctx, 1, nil); err != nil {
		s.Fatal("Failed to detect external monitor: ", err)
	}
	slpOpSetPre := cmdOutput(SlpS0Cmd)
	pkgOpSetOutput := cmdOutput(PkgCstateCmd)
	matchSetPre := (C10PkgPattern).FindStringSubmatch(pkgOpSetOutput)
	if matchSetPre == nil {
		s.Fatal("Failed to match pre PkgCstate value: ", pkgOpSetOutput)
	}
	pkgOpSetPre := matchSetPre[1]
	// Expected sleep before suspending dut
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	stressOut := cmdOutput(SuspendStressCmd)

	suspendErrors := []*regexp.Regexp{SuspndFailurePattern, FrmwreLogErrorPattern, S0ixErrorPattern}
	for _, errmsg := range suspendErrors {
		if !(errmsg).MatchString(string(stressOut)) {
			s.Fatalf("Failed expected %q, but failures are non-zero", errmsg)
		}
	}
	if err := externalDisplayDetection(ctx, 1, nil); err != nil {
		s.Fatal("Failed to detect external monitor: ", err)
	}
	slpOpSetPost := cmdOutput(SlpS0Cmd)
	if slpOpSetPre == slpOpSetPost {
		s.Fatalf("Failed SLP counter value must be different than the value %q noted most recently %q", slpOpSetPre, slpOpSetPost)
	}
	if slpOpSetPost == "0" {
		s.Fatal("Failed SLP counter value must be non-zero, noted is: ", slpOpSetPost)
	}
	pkgOpSetPostOutput := cmdOutput(PkgCstateCmd)
	matchSetPost := (C10PkgPattern).FindStringSubmatch(pkgOpSetPostOutput)
	if matchSetPost == nil {
		s.Fatal("Failed to match post PkgCstate value: ", pkgOpSetPostOutput)
	}
	pkgOpSetPost := matchSetPost[1]
	if pkgOpSetPre == pkgOpSetPost {
		s.Fatalf("Failed Package C10 value %q must be different than value noted earlier %q", pkgOpSetPre, pkgOpSetPost)
	}
	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		s.Fatal("Failed Package C10 should be non-zero")
	}
}

func externalDisplayDetection(ctx context.Context, numberOfDisplays int, regexpPatterns []*regexp.Regexp) error {
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
