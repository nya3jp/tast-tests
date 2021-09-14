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
		s.Log("Waking up DUT")
		if !dut.Connected(ctx) {
			if err := pxy.Servo().SetString(ctx, "power_key", "press"); err != nil {
				s.Fatal("Failed to power state on : ", err)
			}
			if err := dut.WaitConnect(ctx); err != nil {
				s.Log("Unable to wake up DUT. Retrying")
				if err := pxy.Servo().SetString(ctx, "power_key", "press"); err != nil {
					s.Fatal("Failed to power state on : ", err)
				}
				if err := dut.WaitConnect(ctx); err != nil {
					s.Fatal("Failed to wait connect DUT : ", err)
				}
			}
		} else {
			s.Log("DUT is UP")
		}
	}(ctx)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome : ", err)
	}
	// Wait 5 seconds after login before lid close
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed while waiting during sample time: ", err)
	}
	cmdOutput := func(cmd string) string {
		out, err := dut.Conn().CommandContext(ctx, "bash", "-c", cmd).Output()
		if err != nil {
			s.Fatal("Failed to execute slp_s0_residency_usec command: ", err)
		}
		return string(out)
	}
	for i := 1; i <= 10; i++ {
		s.Log("Iteration :", i)
		slpOpSetPre := cmdOutput(SlpS0Cmd)
		pkgOpSetPre := cmdOutput(PkgCstateCmd)
		matchSetPre := (C10PkgPattern).FindStringSubmatch(string(pkgOpSetPre))
		pkgOpSetPre = matchSetPre[1]
		if err := pxy.Servo().SetString(ctx, "lid_open", "no"); err != nil {
			s.Fatal("Failed to close lid : ", err)
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
			s.Fatal("Failed SLP counter value must be different than the value noted most recently")
		}
		if slpOpSetPost == "0" {
			s.Fatal("Failed SLP counter value must be non-zero")
		}
		pkgOpSetPost := cmdOutput(PkgCstateCmd)
		matchSetPost := (C10PkgPattern).FindStringSubmatch(string(pkgOpSetPost))
		pkgOpSetPost = matchSetPost[1]
		if pkgOpSetPre == pkgOpSetPost {
			s.Fatal("Failed Package C10 value must be different than value noted earlier")
		}
		if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
			s.Fatal("Failed Package C10 should be non-zero")
		}
	}

}
