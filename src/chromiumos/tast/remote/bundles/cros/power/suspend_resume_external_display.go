// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SuspendResumeExternalDisplay,
		Desc:         "Verifies suspend-resume with USB type-C display functionality check",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Vars:         []string{"servo"},
		Timeout:      8 * time.Minute,
	})
}

func SuspendResumeExternalDisplay(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	dut := s.DUT()
	servoSpec, ok := s.Var("servo")
	if !ok {
		servoSpec = ""
	}

	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctxForCleanUp)

	var (
		c10PackageRe       = regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
		suspndFailureRe    = "Suspend failures: 0"
		firmwareLogErrorRe = "Firmware log errors: 0"
		s0ixErrorRe        = "s0ix errors: 0"
	)

	const (
		slpS0File     = "/sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		pkgCstateFile = "/sys/kernel/debug/pmc_core/package_cstate_show"
	)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctxForCleanUp)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer client.CloseChrome(ctxForCleanUp, &empty.Empty{})

	if err := extDisplayDetection(ctx, dut, 1); err != nil {
		s.Fatal("Failed to detect external monitor: ", err)
	}

	slpOpSetPre, err := linuxssh.ReadFile(ctx, dut.Conn(), slpS0File)
	if err != nil {
		s.Fatalf("Failed to read %q file: %v", slpS0File, err)
	}

	pkgOpSetOutput, err := linuxssh.ReadFile(ctx, dut.Conn(), pkgCstateFile)
	if err != nil {
		s.Fatalf("Failed to read %q file: %v", pkgCstateFile, err)
	}

	matchSetPre := c10PackageRe.FindStringSubmatch(string(pkgOpSetOutput))
	if matchSetPre == nil {
		s.Fatal("Failed to match pre PkgCstate value: ", pkgOpSetOutput)
	}
	pkgOpSetPre := matchSetPre[1]

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if !dut.Connected(ctx) {
			waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				s.Fatal("Failed to power normal press: ", err)
			}
			if err := dut.WaitConnect(waitCtx); err != nil {
				s.Fatal("Failed to wait connect DUT: ", err)
			}
		}
	}(ctxForCleanUp)

	// Checks poll until no premature wake and/or suspend failures occurs with given poll timeout.
	suspendErrors := []string{suspndFailureRe, firmwareLogErrorRe, s0ixErrorRe}
	if testing.Poll(ctx, func(ctx context.Context) error {
		stressOut, err := dut.Conn().CommandContext(ctx, "suspend_stress_test", "-c", "1").Output()
		if err != nil {
			return errors.Wrap(err, "failed to execute suspend_stress_test command")
		}

		for _, errMsg := range suspendErrors {
			if !strings.Contains(string(stressOut), errMsg) {
				return errors.Errorf("failed was expecting %q, but got failures %s", errMsg, string(stressOut))
			}
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		s.Fatal("Failed to perform suspend_stress_test with zero errors: ", err)
	}

	testing.ContextLog(ctx, "Executing suspend_stress_test command")
	stressOut, err := dut.Conn().CommandContext(ctx, "suspend_stress_test", "-c", "10").Output()
	if err != nil {
		s.Fatal("Failed to execute suspend_stress_test command: ", err)
	}

	for _, errMsg := range suspendErrors {
		if !strings.Contains(string(stressOut), errMsg) {
			s.Fatalf("Failed, expected match %q, but got failures with non-zero: %q", errMsg, string(stressOut))
		}
	}

	if err := extDisplayDetection(ctx, dut, 1); err != nil {
		s.Fatal("Failed to detect external monitor: ", err)
	}

	slpOpSetPost, err := linuxssh.ReadFile(ctx, dut.Conn(), slpS0File)
	if err != nil {
		s.Fatalf("Failed to read %q file after suspend-resume: %v", slpS0File, err)
	}

	if string(slpOpSetPre) == string(slpOpSetPost) {
		s.Fatalf("Failed SLP counter value must be different than the value %q noted most recently %q", string(slpOpSetPre), string(slpOpSetPost))
	}

	if string(slpOpSetPost) == "0" {
		s.Fatal("Failed SLP counter value must be non-zero, noted is: ", slpOpSetPost)
	}

	pkgOpSetPostOutput, err := linuxssh.ReadFile(ctx, dut.Conn(), pkgCstateFile)
	if err != nil {
		s.Fatalf("Failed to read %q file after suspend-resume: %v", pkgCstateFile, err)
	}

	matchSetPost := c10PackageRe.FindStringSubmatch(string(pkgOpSetPostOutput))
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

// extDisplayDetection checks whether connected external display is detected or not.
func extDisplayDetection(ctx context.Context, dut *dut.DUT, numberOfDisplays int) error {
	displayInfoFile := "/sys/kernel/debug/dri/0/i915_display_info"
	displayInfoRe := regexp.MustCompile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := linuxssh.ReadFile(ctx, dut.Conn(), displayInfoFile)
		if err != nil {
			return errors.Wrap(err, "failed to run display info command ")
		}
		matchedString := displayInfoRe.FindAllString(string(out), -1)
		if len(matchedString) != numberOfDisplays {
			return errors.New("connected external display info not found")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "please connect external display as required")
	}
	return nil
}
