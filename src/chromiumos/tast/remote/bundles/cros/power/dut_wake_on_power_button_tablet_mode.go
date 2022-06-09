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
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DUTWakeOnPowerButtonTabletMode, LacrosStatus: testing.LacrosVariantUnknown, Desc: "Verifies waking DUT from S0ix using power button press in tabletmode",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		VarDeps:      []string{"servo"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      10 * time.Minute,
	})
}

func DUTWakeOnPowerButtonTabletMode(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	dut := s.DUT()
	servoSpec := s.RequiredVar("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctxForCleanUp)

	var reC10Package = regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)

	const (
		slpS0File     = "/sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		pkgCstateFile = "/sys/kernel/debug/pmc_core/package_cstate_show"
	)

	// Get the initial tablet_mode_angle settings to restore at the end of test.
	reTabletModeAngle := regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
	out, err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle").Output()
	if err != nil {
		s.Fatal("Failed to retrieve tablet_mode_angle settings: ", err)
	}
	m := reTabletModeAngle.FindSubmatch(out)
	if len(m) != 3 {
		s.Fatalf("Failed to get initial tablet_mode_angle settings: got submatches %+v", m)
	}
	initLidAngle := m[1]
	initTabModeHysteresis := m[2]

	// Set tabletModeAngle to 0 to force the DUT into tablet mode.
	testing.ContextLog(ctx, "Put DUT into tablet mode")
	if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", "0", "0").Run(); err != nil {
		s.Fatal("Failed to set DUT into tablet mode: ", err)
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
	defer client.CloseChrome(ctxForCleanUp, &empty.Empty{})
	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}
		}
		if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", string(initLidAngle), string(initTabModeHysteresis)).Run(); err != nil {
			s.Fatal("Failed to restore tablet_mode_angle to the original settings: ", err)
		}
	}(ctxForCleanUp)

	slpOpSetPre, err := linuxssh.ReadFile(ctx, dut.Conn(), slpS0File)
	if err != nil {
		s.Fatal("Failed to get initial slp s0 value: ", err)
	}

	pkgOpSetOutput, err := linuxssh.ReadFile(ctx, dut.Conn(), pkgCstateFile)
	if err != nil {
		s.Fatal("Failed to get initial PkgCstate value: ", err)
	}

	matchSetPre := reC10Package.FindStringSubmatch(string(pkgOpSetOutput))
	if matchSetPre == nil {
		s.Fatal("Failed to match pre PkgCstate value: ", string(pkgOpSetOutput))
	}

	pkgOpSetPre := matchSetPre[1]
	suspendCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := dut.Conn().CommandContext(suspendCtx, "powerd_dbus_suspend").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		s.Fatal("Failed to power off DUT: ", err)
	}

	sdCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		s.Fatal("Failed to wait for unreachable: ", err)
	}

	if err := powercontrol.PowerOnDutWithRetry(ctx, pxy, dut); err != nil {
		s.Fatal("Failed to power on DUT after suspend: ", err)
	}

	slpOpSetPost, err := linuxssh.ReadFile(ctx, dut.Conn(), slpS0File)
	if err != nil {
		s.Fatal("Failed to get slp s0 value after DUT suspend-resume: ", err)
	}

	if string(slpOpSetPre) == string(slpOpSetPost) {
		s.Errorf("Failed SLP counter value must be different than the value %q noted most recently %q", string(slpOpSetPre), string(slpOpSetPost))
	}

	if string(slpOpSetPost) == "0" {
		s.Error("Failed SLP counter value must be non-zero, noted is: ", slpOpSetPost)
	}

	pkgOpSetPostOutput, err := linuxssh.ReadFile(ctx, dut.Conn(), pkgCstateFile)
	if err != nil {
		s.Fatal("Failed to get PkgCstate value after DUT suspend-resume: ", err)
	}

	matchSetPost := reC10Package.FindStringSubmatch(string(pkgOpSetPostOutput))
	if matchSetPost == nil {
		s.Fatal("Failed to match post PkgCstate value: ", string(pkgOpSetPostOutput))
	}

	pkgOpSetPost := matchSetPost[1]
	if pkgOpSetPre == pkgOpSetPost {
		s.Errorf("Failed Package C10 value %q must be different than value noted earlier %q", pkgOpSetPre, pkgOpSetPost)
	}

	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		s.Error("Failed Package C10 should be non-zero")
	}
}
