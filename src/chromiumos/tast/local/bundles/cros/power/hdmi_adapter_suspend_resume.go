// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"io/ioutil"
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

var (
	// Used for external monitor detection.
	/*
		Example output from /i915_display_info:
		1. One of Pipe B/C/D etc should contain "active=yes" and "hw: active=yes".
			Sample Output:
			[CRTC:91:pipe B]:
			uapi: enable=yes, active=yes, mode="2256x1504": 60 235690 2256 2304 2336 2536 1504 1507 1513 1549 0x48 0x9
			hw: active=yes, adjusted_mode="2256x1504": 60 235690 2256 2304 2336 2536 1504 1507 1513 1549 0x48 0x9
			pipe src size=2256x1504, dither=no, bpp=24
			num_scalers=2, scaler_users=0 scaler_id=-1, scalers[0]: use=no, mode=0, scalers[1]: use=no, mode=0
			[ENCODER:275:DDI A]: connectors:
				[CONNECTOR:276:HDMI-1]
			[PLANE:31:plane 1A]: type=PRI

		2. Connector should contain the information of connector used HDMI/DP.
			Sample output:
			[CONNECTOR:276:HDMI-A-1]: status: connected
			physical dimensions: 280x190mm
			subpixel order: Unknown
			CEA rev: 0

		3. Mode should contain resolution of external display along with FPS.
		    Sample output:
			mode="2256x1504": 60
		    mode="1920x1080": 60
	*/
	displayInfoRe     = regexp.MustCompile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)
	connectorInfoRe   = regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[HDMI]+.*`)
	connectedStatusRe = regexp.MustCompile(`\[CONNECTOR:\d+:HDMI.*status: connected`)
	modesRe           = regexp.MustCompile(`modes:\n.*"1920x1080":.60`)
)

var (
	c10PackageRe       = regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
	suspendFailureRe   = regexp.MustCompile("Suspend failures: 0")
	firmwareLogErrorRe = regexp.MustCompile("Firmware log errors: 0")
	s0ixErrorRe        = regexp.MustCompile("s0ix errors: 0")
	usbDetectionRe     = regexp.MustCompile(`If 0.*Class=.*5000M`)
)

const (
	slpS0File         = "/sys/kernel/debug/pmc_core/slp_s0_residency_usec"
	packageCstateFile = "/sys/kernel/debug/pmc_core/package_cstate_show"
	displayInfoFile   = "/sys/kernel/debug/dri/0/i915_display_info"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HdmiAdapterSuspendResume,
		Desc:         "Verifies USB type-C single port adapter functionality with suspend-resume cycles",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			"power.chameleon_addr",         // Only needed when using chameleon board as extended display.
			"power.chameleon_display_port", // The port connected as extended display. Default is 3.
		},
		Fixture: "chromeLoggedIn",
		Timeout: 8 * time.Minute,
	})
}

func HdmiAdapterSuspendResume(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	// Use chameleon board as extended display. Make sure chameleon is connected.
	chameleonAddr := s.RequiredVar("power.chameleon_addr")
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
	defer dp.Unplug(cleanupCtx)

	if err := dp.Plug(ctx); err != nil {
		s.Fatal("Failed to plug chameleon port: ", err)
	}
	testing.ContextLog(ctx, "Chameleon plugged successfully")

	// Wait for DUT to detect external display.
	if err := dp.WaitVideoInputStable(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for video input on chameleon board: ", err)
	}

	if err := assertAdapterConnected(ctx); err != nil {
		s.Fatal("Failed to detect typec HDMI adapter: ", err)
	}

	if err := assertExternalMonitorConnected(ctx, 1); err != nil {
		s.Fatal("Failed plugging external display: ", err)
	}

	cmdOutput := func(ctx context.Context, file string) string {
		out, err := ioutil.ReadFile(file)
		if err != nil {
			s.Fatalf("Failed to read %q file: %v", file, err)
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

	if err := assertAdapterConnected(ctx); err != nil {
		s.Fatal("Failed to detect typec HDMI adapter after suspend-resume: ", err)
	}

	if err := assertExternalMonitorConnected(ctx, 1); err != nil {
		s.Fatal("Failed plugging external display after suspend-resume: ", err)
	}

	if err := assertSLPCounter(ctx, slpOpSetPre); err != nil {
		s.Fatal("Asserting SLP Counter: ", err)
	}

	if err := assertPackageCState(ctx, pkgOpSetPre); err != nil {
		s.Fatal("Asserting Package C-State failed: ", err)
	}

	if err := assertAdapterConnected(ctx); err != nil {
		s.Fatal("Failed to detect typec HDMI adapter: ", err)
	}

	if err := assertExternalMonitorConnected(ctx, 1); err != nil {
		s.Fatal("Failed to check plug status of external display: ", err)
	}
}

func assertSLPCounter(ctx context.Context, slpOpSetPre string) error {
	slpOpSetPost, err := ioutil.ReadFile(slpS0File)
	if err != nil {
		return errors.Wrapf(err, "failed to read %q file", slpS0File)
	}
	if slpOpSetPre == string(slpOpSetPost) {
		return errors.Errorf("failed SLP counter value must be different than the value %q noted most recently %q", slpOpSetPre, slpOpSetPost)
	}
	if string(slpOpSetPost) == "0" {
		return errors.Errorf("failed SLP counter value must be non-zero, noted is: %q", slpOpSetPost)
	}
	return nil
}

func assertPackageCState(ctx context.Context, pkgOpSetPre string) error {
	pkgOpSetPostOutput, err := ioutil.ReadFile(packageCstateFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read %q file", packageCstateFile)
	}
	matchSetPost := c10PackageRe.FindStringSubmatch(string(pkgOpSetPostOutput))
	if matchSetPost == nil {
		return errors.Errorf("failed to match post PkgCstate value: %q", pkgOpSetPostOutput)
	}
	pkgOpSetPost := matchSetPost[1]
	if pkgOpSetPre == pkgOpSetPost {
		return errors.Errorf("failed Package C10 value %q must be different than value %q noted most recently", pkgOpSetPre, pkgOpSetPost)
	}
	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		return errors.Errorf("failed Package C10 = want non-zero, got %s", pkgOpSetPost)
	}
	return nil
}

func assertExternalMonitorConnected(ctx context.Context, numberOfDisplays int) error {
	displayInfoPatterns := []*regexp.Regexp{connectorInfoRe, connectedStatusRe, modesRe}
	displCount, err := graphics.NumberOfOutputsConnected(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get connected displays ")
	}
	if displCount < 2 {
		return errors.New("external display is not connected")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := ioutil.ReadFile(displayInfoFile)
		if err != nil {
			return errors.Wrap(err, "failed to run display info command ")
		}
		matchedString := displayInfoRe.FindAllString(string(out), -1)
		if len(matchedString) != numberOfDisplays {
			return errors.New("connected external display info not found")
		}

		for _, pattern := range displayInfoPatterns {
			if !pattern.MatchString(string(out)) {
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

func assertAdapterConnected(ctx context.Context) error {
	out, err := testexec.CommandContext(ctx, "lsusb", "-t").Output()
	if err != nil {
		return errors.Wrap(err, "failed to execute lsusb command")
	}
	if !usbDetectionRe.MatchString(string(out)) {
		return errors.New("typec HDMI adapter is not connected")
	}
	return nil
}
