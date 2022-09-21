// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LidCloseOpenSuspendStress,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies lid close-open suspend stress test",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"servo", "intel.LidCloseOpenSuspendStress.iterations"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.X86()),
		Timeout:      1 * time.Hour,
	})
}

func LidCloseOpenSuspendStress(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	dut := s.DUT()

	servoSpec := s.RequiredVar("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			if err := pxy.Servo().OpenLid(ctx); err != nil {
				s.Error("Failed to lid open at cleanup: ", err)
			}
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Error("Failed to power-on DUT at cleanup: ", err)
			}
		}
	}(cleanupCtx)

	// Performs Chrome login.
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login Chrome: ", err)
	}

	firmwareHelper := &firmware.Helper{Servo: pxy.Servo()}

	iter, err := strconv.Atoi(s.RequiredVar("intel.LidCloseOpenSuspendStress.iterations"))
	if err != nil {
		s.Fatal("Failed to convert string to integer: ", err)
	}

	for i := 1; i <= iter; i++ {
		s.Logf("Iteration: %d/%d", i, iter)
		if err := pxy.Servo().CloseLid(ctx); err != nil {
			s.Fatal("Failed to close lid: ", err)
		}

		sdCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
		defer cancel()
		if err := dut.WaitUnreachable(sdCtx); err != nil {
			s.Fatal("Failed to wait DUT to become unreachable: ", err)
		}

		if err := powercontrol.WaitForSuspendState(ctx, firmwareHelper); err != nil {
			s.Fatal("Failed to wait for DUT suspend state: ", err)
		}

		if err := pxy.Servo().OpenLid(ctx); err != nil {
			s.Fatal("Failed to open lid: ", err)
		}

		waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		if err := dut.WaitConnect(waitCtx); err != nil {
			s.Fatal("Failed to wait connect DUT: ", err)
		}
	}
}
