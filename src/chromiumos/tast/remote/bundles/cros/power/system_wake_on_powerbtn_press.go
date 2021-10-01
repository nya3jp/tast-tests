// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SystemWakeOnPowerbtnPress,
		Desc:         "Test waking DUT from S0ix using power button stress",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Vars:         []string{"servo"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      10 * time.Minute,
	})
}

func SystemWakeOnPowerbtnPress(ctx context.Context, s *testing.State) {
	dut := s.DUT()
	var c10PkgPattern = regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
	const (
		slpS0Cmd     = "cat /sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		pkgCstateCmd = "cat /sys/kernel/debug/pmc_core/package_cstate_show"
		iter         = 10
	)
	servoHostPort, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoHostPort, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer client.CloseChrome(ctx, &empty.Empty{})
	defer func() {
		if !dut.Connected(ctx) {
			if err := powerOnDut(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}
		}
	}()
	cmdOutput := func(cmd string) string {
		out, err := dut.Conn().CommandContext(ctx, "bash", "-c", cmd).Output()
		if err != nil {
			s.Fatalf("Failed to execute %s command: %v", cmd, err)
		}
		return string(out)
	}

	for i := 1; i <= iter; i++ {
		s.Logf("Iteration: %d/%d", i, iter)
		slpOpSetPre := cmdOutput(slpS0Cmd)
		pkgOpSetOutput := cmdOutput(pkgCstateCmd)
		matchSetPre := (c10PkgPattern).FindStringSubmatch(pkgOpSetOutput)
		if matchSetPre == nil {
			s.Fatal("Failed to match pre PkgCstate value: ", pkgOpSetOutput)
		}
		pkgOpSetPre := matchSetPre[1]
		powerOffCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err := dut.Conn().CommandContext(powerOffCtx, "powerd_dbus_suspend").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			s.Fatal("Failed to power off DUT: ", err)
		}
		sdCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err := dut.WaitUnreachable(sdCtx); err != nil {
			s.Fatal("Failed to wait for unreachable: ", err)
		}
		if err := powerOnDut(ctx, pxy, dut); err != nil {
			s.Fatal("Failed to power on DUT after suspend: ", err)
		}
		slpOpSetPost := cmdOutput(slpS0Cmd)
		if slpOpSetPre == slpOpSetPost {
			s.Fatalf("Failed SLP counter value must be different than the value %q noted most recently %q", slpOpSetPre, slpOpSetPost)
		}
		if slpOpSetPost == "0" {
			s.Fatal("Failed SLP counter value must be non-zero, noted is: ", slpOpSetPost)
		}
		pkgOpSetPostOutput := cmdOutput(pkgCstateCmd)
		matchSetPost := (c10PkgPattern).FindStringSubmatch(pkgOpSetPostOutput)
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
}

func powerOnDut(ctx context.Context, pxy *servo.Proxy, dut *dut.DUT) error {
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Wrap(err, "failed to power normal press")
	}
	if err := dut.WaitConnect(waitCtx); err != nil {
		testing.ContextLog(ctx, "Unable to wake up DUT. Retrying")
		if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			return errors.Wrap(err, "failed to power normal press")
		}
		if err := dut.WaitConnect(waitCtx); err != nil {
			return errors.Wrap(err, "failed to wait connect DUT")
		}
	}
	return nil
}
