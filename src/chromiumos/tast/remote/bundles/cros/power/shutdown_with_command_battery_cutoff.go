// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShutdownWithCommandBatteryCutoff, LacrosStatus: testing.LacrosVariantUnneeded, Desc: "Verifies that system comes back after executing shutdown command with battery cutoff",
		Contacts:     []string{"timvp@google.com", "cros-fw-engprod@google.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome", "reboot"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      5 * time.Minute,
	})
}

func ShutdownWithCommandBatteryCutoff(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	dut := s.DUT()
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctxForCleanUp)

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		testing.ContextLog(ctx, "Rebooting EC to restore battery connection")
		if err := pxy.Servo().RunECCommand(ctx, "reboot"); err != nil {
			s.Fatal("Failed to reboot EC: ", err)
		}
		// Wait a little at the end of the test to make sure the EC finishes booting before the next test runs.
		defer func() {
			s.Log("Waiting for boot to finish")
			if err := testing.Sleep(ctx, 20*time.Second); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}
		}()
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to wake up DUT at cleanup: ", err)
			}
		}
	}(ctxForCleanUp)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctxForCleanUp)

	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	// Set tabletModeAngle to 0 to force the DUT into tablet mode.
	testing.ContextLog(ctx, "Cut off the battery")
	if err := dut.Conn().CommandContext(ctx, "ectool", "batterycutoff").Run(); err != nil {
		s.Fatal("Failed to issue `ectool batterycutoff`: ", err)
	}

	powerOffCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := dut.Conn().CommandContext(powerOffCtx, "shutdown", "-h", "now").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		s.Fatal("Failed to execute shutdown command: ", err)
	}

	sdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		s.Fatal("Failed to shutdown DUT: ", err)
	}

	if err := powercontrol.ValidateG3PowerState(ctx, pxy); err != nil {
		s.Fatal("Failed to enter G3 after shutdown: ", err)
	}

	if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
		s.Fatal("Failed to wake up DUT: ", err)
	}

	if _, err := powercontrol.ChromeLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}
}
