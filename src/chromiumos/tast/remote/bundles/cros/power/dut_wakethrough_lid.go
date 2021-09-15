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
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DUTWakethroughLid,
		Desc:         "Verifies that system comes back after lid open in iterations",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"servo"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      4 * time.Minute,
	})
}

func DUTWakethroughLid(ctx context.Context, s *testing.State) {
	c10PkgPattern := regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
	const (
		slpS0Cmd     = "cat /sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		pkgCstateCmd = "cat /sys/kernel/debug/pmc_core/package_cstate_show"
	)
	dut := s.DUT()
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	// Cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		pxy.Servo().SetString(ctx, "lid_open", "yes")
		if dut.Connected(ctx) {
			s.Log("DUT is up")
			return
		}
		if err := pxy.Servo().SetString(ctx, "power_key", "press"); err != nil {
			s.Fatal("Failed to power state on: ", err)
		}
		if err := dut.WaitConnect(ctx); err != nil {
			s.Log("Unable to wake up DUT. Retrying")
			if err := pxy.Servo().SetString(ctx, "power_key", "press"); err != nil {
				s.Fatal("Failed to power state on: ", err)
			}
			if err := dut.WaitConnect(ctx); err != nil {
				s.Fatal("Failed to wait connect DUT: ", err)
			}
		}
	}(cleanupCtx)

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
	// Wait 5 seconds after login before lid close.
	// TODO: Need to remove testing.Sleep().
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed while waiting during sample time: ", err)
	}
	cmdOutput := func(cmd string) string {
		out, err := dut.Conn().CommandContext(ctx, "bash", "-c", cmd).Output(testexec.DumpLogOnError)
		if err != nil {
			s.Fatal("Failed to execute command: ", err)
		}
		return strings.Trim(string(out), "\n")
	}
	const numIterations = 10
	for i := 1; i <= numIterations; i++ {
		s.Logf("Iteration %d  %d", i, numIterations)
		slpOpSetPre := cmdOutput(slpS0Cmd)
		pkgOpSetOutput := cmdOutput(pkgCstateCmd)
		matchSetPre := (c10PkgPattern).FindStringSubmatch(pkgOpSetOutput)
		if matchSetPre == nil {
			s.Fatal("Failed to match pre PkgCstate value: ", pkgOpSetOutput)
		}
		pkgOpSetPre := matchSetPre[1]
		s.Log("Closing lid")
		if err := pxy.Servo().SetString(ctx, "lid_open", "no"); err != nil {
			s.Fatal("Failed to close lid: ", err)
		}
		if err := dut.WaitUnreachable(ctx); err != nil {
			s.Fatal("Failed to wait DUT to become unreachable: ", err)
		}
		s.Log("Opening lid")
		if err := pxy.Servo().SetString(ctx, "lid_open", "yes"); err != nil {
			s.Fatal("Failed to open lid: ", err)
		}
		if err := dut.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to wake up DUT: ", err)
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
