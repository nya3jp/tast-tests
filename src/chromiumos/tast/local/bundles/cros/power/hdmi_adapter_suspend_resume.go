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
		Func:         HdmiAdapterSuspendResume,
		Desc:         "Verifies USB type-C single port adapter functionality with suspend-resume cycles",
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

func HdmiAdapterSuspendResume(ctx context.Context, s *testing.State) {
	var (
		c10PackageRe       = regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
		connectorInfoRe    = regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[HDMI]+.*`)
		connectedStatusRe  = regexp.MustCompile(`\[CONNECTOR:\d+:HDMI.*status: connected`)
		modesRe            = regexp.MustCompile(`modes:\n.*"1920x1080":.60`)
		suspendFailureRe   = regexp.MustCompile("Suspend failures: 0")
		firmwareLogErrorRe = regexp.MustCompile("Firmware log errors: 0")
		s0ixErrorRe        = regexp.MustCompile("s0ix errors: 0")
		portID             = 3 // Use default port 3 for display.
	)
	const (
		slpS0File         = "/sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		packageCstateFile = "/sys/kernel/debug/pmc_core/package_cstate_show"
	)
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()
	chameleonAddr, ok := s.Var("power.chameleon_addr")
	if !ok {
		s.Fatal("Failed please provide chameleon IP address")
	}
	// Use chameleon board as extended display. Make sure chameleon is connected.
	che, err := chameleon.New(ctx, chameleonAddr)
	if err != nil {
		s.Fatal("Failed to connect to chameleon board: ", err)
	}
	defer che.Close(cleanupCtx)

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
	// Cleanup
	defer dp.Unplug(cleanupCtx)

	if err := dp.Plug(ctx); err != nil {
		s.Fatal("Failed to plug chameleon port: ", err)
	}
	testing.ContextLog(ctx, "Chameleon plugged successfully")

	// Wait for DUT to detect external display.
	if err := dp.WaitVideoInputStable(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for video input on chameleon board: ", err)
	}

	if err := adapterDetection(ctx); err != nil {
		s.Fatal("Failed to detect typec HDMI adapter: ", err)
	}

	displayInfoPatterns := []*regexp.Regexp{connectorInfoRe, connectedStatusRe, modesRe}
	if err := extMonitorDetection(ctx, 1, displayInfoPatterns); err != nil {
		s.Fatal("Failed plugging external display: ", err)
	}

	cmdOutput := func(ctx context.Context, cmd string) string {
		out, err := testexec.CommandContext(ctx, "cat", cmd).Output()
		if err != nil {
			s.Fatalf("Failed to execute 'cat %s' command: %v", cmd, err)
		}
		return string(out)
	}

	slpOpSetPre := cmdOutput(ctx, slpS0File)
	pkgOpSetOutput := cmdOutput(ctx, packageCstateFile)
	matchSetPre := c10PackageRe.FindStringSubmatch(pkgOpSetOutput)
	if matchSetPre == nil {
		s.Fatal("Failed to match pre PkgCstate value: ", pkgOpSetOutput)
	}
	pkgOpSetPre := matchSetPre[1]
	testing.ContextLog(ctx, "Executing suspend_stress_test for 10 cycles")
	stressOut, err := testexec.CommandContext(ctx, "suspend_stress_test", "-c", "10").Output()
	if err != nil {
		s.Fatal("Failed to execute suspend_stress_test command: ", err)
	}

	suspendErrors := []*regexp.Regexp{suspendFailureRe, firmwareLogErrorRe, s0ixErrorRe}
	for _, errmsg := range suspendErrors {
		if !(errmsg.MatchString(string(stressOut))) {
			s.Fatalf("Failed expected %q, but failures are non-zero", errmsg)
		}
	}
	if err := adapterDetection(ctx); err != nil {
		s.Fatal("Failed to detect typec HDMI adapter after suspend-resume: ", err)
	}
	if err := extMonitorDetection(ctx, 1, displayInfoPatterns); err != nil {
		s.Fatal("Failed plugging external display after suspend-resume: ", err)
	}

	slpOpSetPost := cmdOutput(ctx, slpS0File)
	if slpOpSetPre == slpOpSetPost {
		s.Fatalf("Failed SLP counter value must be different than the value %q noted most recently %q", slpOpSetPre, slpOpSetPost)
	}
	if slpOpSetPost == "0" {
		s.Fatal("Failed SLP counter value must be non-zero, noted is: ", slpOpSetPost)
	}
	pkgOpSetPostOutput := cmdOutput(ctx, packageCstateFile)
	matchSetPost := c10PackageRe.FindStringSubmatch(pkgOpSetPostOutput)
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
	if err := adapterDetection(ctx); err != nil {
		s.Fatal("Failed to detect typec HDMI adapter: ", err)
	}
	if err := extMonitorDetection(ctx, 1, displayInfoPatterns); err != nil {
		s.Fatal("Failed to check plug status of external display: ", err)
	}
}

func extMonitorDetection(ctx context.Context, numberOfDisplays int, regexpPatterns []*regexp.Regexp) error {
	const DisplayInfoFile = "/sys/kernel/debug/dri/0/i915_display_info"
	displCount, err := graphics.NumberOfOutputsConnected(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get connected displays ")
	}

	var displayInfoRe = regexp.MustCompile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)
	if displCount < 2 {
		return errors.New("external display is not connected")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "cat", DisplayInfoFile).Output()

		if err != nil {
			return errors.Wrap(err, "failed to run display info command ")
		}
		matchedString := displayInfoRe.FindAllString(string(out), -1)
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

func adapterDetection(ctx context.Context) error {
	usbDetectionRe := regexp.MustCompile(`Class=.*(480M|5000M|10G|20G)`)
	out, err := testexec.CommandContext(ctx, "lsusb", "-t").Output()
	if err != nil {
		return errors.Wrap(err, "failed to execute lsusb command")
	}
	if !usbDetectionRe.MatchString(string(out)) {
		return errors.New("typec HDMI adapter is not connected")
	}
	return nil
}
