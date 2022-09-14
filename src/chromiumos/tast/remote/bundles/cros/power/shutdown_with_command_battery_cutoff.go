// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/remote/firmware/fixture"
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
		Attr:         []string{"group:mainline", "informational", "group:firmware", "firmware_unstable"},
		Fixture:      fixture.NormalMode,
		Vars:         []string{"servo"},
		Timeout:      5 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery(),
			hwdep.Model(
				"skyrim15w",
				"skyrim15w360",
			)),
	})
}

func ShutdownWithCommandBatteryCutoff(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	dut := s.DUT()
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	// Verify AC is attached so the DUT is powered after the battery is cut off.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		chargerAttached, err := h.Servo.GetChargerAttached(ctx)
		if err != nil {
			s.Fatal("Error checking whether charger is attached: ", err)
		}
		if !chargerAttached {
			s.Fatal("Charger not attached")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}); err != nil {
		s.Fatal(err, "Failed to check for charger")
	}
	s.Log("Charger is present")

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		testing.ContextLog(ctx, "Rebooting EC to restore battery connection")
		if err := pxy.Servo().RunECCommand(ctx, "reboot"); err != nil {
			s.Fatal("Failed to reboot EC: ", err)
		}
		// Wait a little at the end of the test to make sure the EC finishes booting before the next test runs.
		testing.ContextLog(ctx, "Waiting for EC to boot")
		defer func() {
			s.Log("Waiting for boot to finish")
			if err := testing.Sleep(ctx, 20*time.Second); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}
		}()
		testing.ContextLog(ctx, "Attempting to connect to DUT")
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to wake up DUT at cleanup: ", err)
			}
		}
	}(cleanupCtx)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(cleanupCtx)

	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	// Cut off the battery to simulate booting without a battery.
	testing.ContextLog(ctx, "Cut off the battery")
	if err := dut.Conn().CommandContext(ctx, "ectool", "batterycutoff").Run(); err != nil {
		s.Fatal("Failed to issue `ectool batterycutoff`: ", err)
	}

	testing.ContextLog(ctx, "Shutting down the AP")
	powerOffCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := dut.Conn().CommandContext(powerOffCtx, "shutdown", "-h", "now").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		s.Fatal("Failed to execute shutdown command: ", err)
	}

	testing.ContextLog(ctx, "Waiting for AP to shutdown")
	sdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		s.Fatal("Failed to shutdown DUT: ", err)
	}

	testing.ContextLog(ctx, "Validating G3 power state")
	if err := powercontrol.ValidateG3PowerState(ctx, pxy); err != nil {
		s.Fatal("Failed to enter G3 after shutdown: ", err)
	}

	testing.ContextLog(ctx, "Powering on DUT")
	if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
		s.Fatal("Failed to wake up DUT: ", err)
	}

	testing.ContextLog(ctx, "Logging into ChromeOS")
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}

	// We expect 'err' to be non-nil (an error to be generated) since
	// 'ectool batterycutoff' was issued and the EC can no longer get good
	// values from the battery, including it's current charge status.
	_, err = h.Servo.GetBatteryChargeMAH(ctx)
	if err == nil {
		s.Fatal("Failed: battery is not cut off after power cycle")
	}
}
