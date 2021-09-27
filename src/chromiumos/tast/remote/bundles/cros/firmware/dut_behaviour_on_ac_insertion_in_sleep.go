// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DUTBehaviourOnACInsertionInSleep,
		Desc:         "Verifies that system comes back after from sleep after AC insertion",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com", "cros-fw-engprod@google.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome", "reboot"},
		Vars:         []string{"servo"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.NormalMode,
	})
}

func DUTBehaviourOnACInsertionInSleep(ctx context.Context, s *testing.State) {
	const (
		// cmdTimeout is a short duration used for sending commands.
		cmdTimeout = 3 * time.Second
	)
	dut := s.DUT()
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create config: ", err)
	}
	var C10PkgPattern = regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
	const (
		SlpS0Cmd     = "cat /sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		PkgCstateCmd = "cat /sys/kernel/debug/pmc_core/package_cstate_show"
	)
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	getChargerPollOptions := testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: 250 * time.Millisecond,
	}
	defer func() {
		s.Log("Stopping power supply")
		if err := h.SetDUTPower(ctx, true); err != nil {
			s.Fatal("Failed to connect charger: ", err)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
				return err
			} else if !attached {
				return errors.New("charger is not attached")
			}
			return nil
		}, &getChargerPollOptions); err != nil {
			s.Fatal("Check for charger failed: ", err)
		}
	}()
	s.Log("Stopping power supply")
	if err := h.SetDUTPower(ctx, false); err != nil {
		s.Fatal("Failed to remove charger: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
			return err
		} else if attached {
			return errors.New("charger is still attached - use Servo V4 Type-C or supply RPM vars")
		}
		return nil
	}, &getChargerPollOptions); err != nil {
		s.Fatal("Check for charger failed: ", err)
	}

	cmdOutput := func(cmd string) string {
		out, err := dut.Conn().CommandContext(ctx, "bash", "-c", cmd).Output()
		if err != nil {
			s.Fatal("Failed to execute slp_s0_residency_usec command: ", err)
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
	powerOffCtx, cancel := context.WithTimeout(ctx, cmdTimeout)
	defer cancel()
	if err := h.DUT.Conn().CommandContext(powerOffCtx, "powerd_dbus_suspend").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		s.Fatal("Failed to power off DUT: ", err)
	}
	sdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		s.Fatal("Failed to wait for unreachable: ", err)
	}
	s.Log("Attaching power supply")
	if err := h.SetDUTPower(ctx, true); err != nil {
		s.Fatal("Failed to attach charger: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
			return err
		} else if !attached {
			return errors.New("charger is not attached")
		}
		return nil
	}, &getChargerPollOptions); err != nil {
		s.Fatal("Failed to attach charger: ", err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := dut.WaitConnect(waitCtx); err != nil {
		s.Fatal("Failed to wait connect DUT at suspend state after insertion AC charger: ", err)
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
