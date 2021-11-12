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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DUTBehaviorOnACInsertionTabletMode,
		Desc:         "Verifies that system comes back from sleep after AC insertion in tabletmode",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome", "reboot"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      10 * time.Minute,
		Fixture:      fixture.NormalMode,
	})
}

func DUTBehaviorOnACInsertionTabletMode(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	dut := s.DUT()
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create config: ", err)
	}

	const (
		// cmdTimeout is a short duration used for sending commands.
		cmdTimeout = 3 * time.Second
	)

	c10PackageRe := regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
	const (
		slpS0File     = "/sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		pkgCstateFile = "/sys/kernel/debug/pmc_core/package_cstate_show"
	)
	// Get the initial tablet_mode_angle settings to restore at the end of test.
	re := regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
	out, err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle").Output()
	if err != nil {
		s.Fatal("Failed to retrieve tablet_mode_angle settings: ", err)
	}
	m := re.FindSubmatch(out)
	if len(m) != 3 {
		s.Fatalf("Failed to get initial tablet_mode_angle settings: got submatches %+v", m)
	}
	initLidAngle := m[1]
	initHys := m[2]
	// Set tabletModeAngle to 0 to force the DUT into tablet mode.
	testing.ContextLog(ctx, "Put DUT into tablet mode")
	if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", "0", "0").Run(); err != nil {
		s.Fatal("Failed to set DUT into tablet mode: ", err)
	}

	getChargerPollOptions := testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: 250 * time.Millisecond,
	}
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

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctxForCleanUp)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Performing cleanup")
		if !dut.Connected(ctx) {
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				s.Fatal("Failed to power normal press: ", err)
			}
			if err := dut.WaitConnect(ctx); err != nil {
				s.Fatal("Failed to wait connect DUT: ", err)
			}
		}

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
		if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", string(initLidAngle), string(initHys)).Run(); err != nil {
			s.Fatal("Failed to restore tablet_mode_angle to the original settings: ", err)
		}
	}(ctxForCleanUp)

	cmdOutput := func(cmdFile string) string {
		out, err := dut.Conn().CommandContext(ctx, "cat", cmdFile).Output()
		if err != nil {
			s.Fatalf("Failed to execute 'cat %s' command: %v", cmdFile, err)
		}
		return string(out)
	}
	slpOpSetPre := cmdOutput(slpS0File)
	pkgOpSetOutput := cmdOutput(pkgCstateFile)
	matchSetPre := c10PackageRe.FindStringSubmatch(pkgOpSetOutput)
	if matchSetPre == nil {
		s.Fatal("Failed to match pre PkgCstate value: ", pkgOpSetOutput)
	}
	pkgOpSetPre := matchSetPre[1]
	powerOffCtx, cancel := context.WithTimeout(ctx, cmdTimeout)
	defer cancel()

	if err := h.DUT.Conn().CommandContext(powerOffCtx, "powerd_dbus_suspend").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		s.Fatal("Failed to power off DUT: ", err)
	}
	sdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
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

	slpOpSetPost := cmdOutput(slpS0File)
	if slpOpSetPre == slpOpSetPost {
		s.Fatalf("Failed SLP counter value %q must be different than the value %q noted earlier", slpOpSetPost, slpOpSetPre)
	}
	if slpOpSetPost == "0" {
		s.Fatalf("Failed SLP counter value = got %v, want non-zero", slpOpSetPost)
	}
	pkgOpSetPostOutput := cmdOutput(pkgCstateFile)
	matchSetPost := c10PackageRe.FindStringSubmatch(pkgOpSetPostOutput)
	if matchSetPost == nil {
		s.Fatal("Failed to match post PkgCstate value: ", pkgOpSetPostOutput)
	}
	pkgOpSetPost := matchSetPost[1]
	if pkgOpSetPre == pkgOpSetPost {
		s.Fatalf("Failed Package C10 value %q must be different than the value %q noted earlier", pkgOpSetPost, pkgOpSetPre)
	}
	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		s.Fatalf("Failed Package C10 value = got %v, want non-zero", pkgOpSetPost)
	}
}
