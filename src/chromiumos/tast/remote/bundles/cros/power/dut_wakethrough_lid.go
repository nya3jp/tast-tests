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
	})
}

func DUTWakethroughLid(ctx context.Context, s *testing.State) {
	C10PkgPattern := regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
	const (
		SlpS0Cmd     = "cat /sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		PkgCstateCmd = "cat /sys/kernel/debug/pmc_core/package_cstate_show"
	)
	dut := s.DUT()
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	// Cleanup.
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
	}(ctx)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
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
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed while waiting during sample time: ", err)
	}
	cmdOutput := func(cmd string) string {
		out, err := dut.Conn().CommandContext(ctx, "bash", "-c", cmd).Output()
		if err != nil {
			s.Fatal("Failed to execute command: ", err)
		}
		return strings.Trim(string(out), "\n")
	}
	const numIterations = 10
	for i := 1; i <= numIterations; i++ {
		s.Logf("Iteration %d of %d", i, numIterations)
		slpOpSetPre := cmdOutput(SlpS0Cmd)
		pkgOpSetOutput := cmdOutput(PkgCstateCmd)
		matchSetPre := (C10PkgPattern).FindStringSubmatch(pkgOpSetOutput)
		if len(matchSetPre) == 0 {
			s.Fatal("Failed to match pre PkgCstate value: ", err)
		}
		pkgOpSetPre := matchSetPre[1]
		if err := pxy.Servo().SetString(ctx, "lid_open", "no"); err != nil {
			s.Fatal("Failed to close lid: ", err)
		}
		if err := dut.WaitUnreachable(ctx); err != nil {
			s.Fatal("Failed to wait connect DUT: ", err)
		}
		s.Log("Opening lid")
		if err := pxy.Servo().SetString(ctx, "lid_open", "yes"); err != nil {
			s.Fatal("Failed to open lid: ", err)
		}
		if err := dut.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to wake up DUT: ", err)
		}
		slpOpSetPost := cmdOutput(SlpS0Cmd)
		if slpOpSetPre == slpOpSetPost {
			s.Fatalf("Failed SLP counter value must be different than the value %q noted most recently %q", slpOpSetPre, slpOpSetPost)
		}
		if slpOpSetPost == "0" {
			s.Fatal("Failed SLP counter value must be non-zero")
		}
		pkgOpSetPostOutput := cmdOutput(PkgCstateCmd)
		matchSetPost := (C10PkgPattern).FindStringSubmatch(string(pkgOpSetPostOutput))
		if len(matchSetPost) == 0 {
			s.Fatal("Failed to match post PkgCstate value: ", err)
		}
		pkgOpSetPost := matchSetPost[1]
		if pkgOpSetPre == pkgOpSetPost {
			s.Fatal("Failed Package C10 value must be different than value noted earlier")
		}
		if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
			s.Fatal("Failed Package C10 should be non-zero")
		}
	}

}
