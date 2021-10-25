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
		Func:         SuspendResumeUSBCDisplay,
		Desc:         "Verifies suspend-resume with USB type-C display functionality check",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Vars:         []string{"servo"},
		Timeout:      8 * time.Minute,
	})
}

func SuspendResumeUSBCDisplay(ctx context.Context, s *testing.State) {
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
		c10PackageRe = regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
	)

	const (
		slpS0File             = "/sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		pkgCstateFile         = "/sys/kernel/debug/pmc_core/package_cstate_show"
		zeroSuspendFailures   = "Suspend failures: 0"
		zeroFirmwareLogErrors = "Firmware log errors: 0"
		zeroS0ixErrors        = "s0ix errors: 0"
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

	slpOpSetPreBytes, err := linuxssh.ReadFile(ctx, dut.Conn(), slpS0File)
	if err != nil {
		s.Fatal("Failed to get SLP counter value: ", err)
	}

	slpOpSetPre, err := strconv.Atoi(strings.TrimSpace(string(slpOpSetPreBytes)))
	if err != nil {
		s.Fatal("Failed to convert type string to integer: ", err)
	}

	pkgOpSetOutput, err := linuxssh.ReadFile(ctx, dut.Conn(), pkgCstateFile)
	if err != nil {
		s.Fatal("Failed to get package cstate value: ", err)
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

	// Poll for running suspend_stress_test until no premature wake and/or suspend failures occurs with given poll timeout.
	testing.ContextLog(ctx, "Wait for a suspend test without failures")
	zeroSuspendErrors := []string{zeroSuspendFailures, zeroFirmwareLogErrors, zeroS0ixErrors}
	if testing.Poll(ctx, func(ctx context.Context) error {
		stressOut, err := dut.Conn().CommandContext(ctx, "suspend_stress_test", "-c", "1").Output()
		if err != nil {
			return errors.Wrap(err, "failed to execute suspend_stress_test command")
		}

		for _, errMsg := range zeroSuspendErrors {
			if !strings.Contains(string(stressOut), errMsg) {
				return errors.Errorf("expect zero failures for %q, got %q", errMsg, string(stressOut))
			}
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		s.Fatal("Failed to perform suspend_stress_test with zero errors: ", err)
	}

	testing.ContextLog(ctx, "Run: suspend_stress_test -c 10")
	stressOut, err := dut.Conn().CommandContext(ctx, "suspend_stress_test", "-c", "10").Output()
	if err != nil {
		s.Fatal("Failed to execute suspend_stress_test command: ", err)
	}

	for _, errMsg := range zeroSuspendErrors {
		if !strings.Contains(string(stressOut), errMsg) {
			s.Fatalf("Failed: expect zero failures for %q, got %q", errMsg, string(stressOut))
		}
	}

	if err := extDisplayDetection(ctx, dut, 1); err != nil {
		s.Fatal("Failed to detect external monitor: ", err)
	}

	slpOpSetPostBytes, err := linuxssh.ReadFile(ctx, dut.Conn(), slpS0File)
	if err != nil {
		s.Fatal("Failed to get SLP counter value after suspend-resume: ", err)
	}

	slpOpSetPost, err := strconv.Atoi(strings.TrimSpace(string(slpOpSetPostBytes)))
	if err != nil {
		s.Fatal("Failed to convert type string to integer: ", err)
	}

	if slpOpSetPre == slpOpSetPost {
		s.Fatalf("Failed: SLP counter value %q should be different from the one before suspend %q", slpOpSetPost, slpOpSetPre)
	}

	if slpOpSetPost == 0 {
		s.Fatal("Failed SLP counter value must be non-zero, got: ", slpOpSetPost)
	}

	pkgOpSetPostOutput, err := linuxssh.ReadFile(ctx, dut.Conn(), pkgCstateFile)
	if err != nil {
		s.Fatal("Failed to get package cstate value after suspend-resume: ", err)
	}

	matchSetPost := c10PackageRe.FindStringSubmatch(string(pkgOpSetPostOutput))
	if matchSetPost == nil {
		s.Fatal("Failed to match post PkgCstate value: ", pkgOpSetPostOutput)
	}

	pkgOpSetPost := matchSetPost[1]
	if pkgOpSetPre == pkgOpSetPost {
		s.Fatalf("Failed: Package C10 value %q must be different than value noted earlier %q", pkgOpSetPre, pkgOpSetPost)
	}

	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		s.Fatal("Failed: Package C10 should be non-zero")
	}
}

// extDisplayDetection checks whether an external display is detected or not.
func extDisplayDetection(ctx context.Context, dut *dut.DUT, numberOfDisplays int) error {
	const displayInfoFile = "/sys/kernel/debug/dri/0/i915_display_info"
	displayInfoRe := regexp.MustCompile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := linuxssh.ReadFile(ctx, dut.Conn(), displayInfoFile)
		if err != nil {
			return errors.Wrap(err, "failed to get display info")
		}
		matchedString := displayInfoRe.FindAllString(string(out), -1)
		if actual := len(matchedString); actual != numberOfDisplays {
			return errors.Errorf("Unexpected number of external display: want %d, actual %d", numberOfDisplays, actual)
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		return err
	}
	return nil
}
