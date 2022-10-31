// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShutdownPwrbuttonStress, LacrosStatus: testing.LacrosVariantUnknown, Desc: "Verifies stress test shutdown using power button",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com", "cros-fw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Vars: []string{"servo",
			"power.iterations",
			"power.mode", // Optional. Expecting "tablet". By defaault power.mode will be "clamshell".
		},
		Attr:    []string{"group:mainline", "informational"},
		Timeout: 40 * time.Minute,
	})
}

// ShutdownPwrbuttonStress Perform shutdown using power button.
func ShutdownPwrbuttonStress(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	dut := s.DUT()
	servoHostPort, ok := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoHostPort, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	defer pxy.Close(ctxForCleanUp)
	defaultIter := 2
	newIter, ok := s.Var("power.iterations")
	if ok {
		defaultIter, _ = strconv.Atoi(newIter)
	}

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

	defaultMode := "clamshell"
	if mode, ok := s.Var("power.mode"); ok {
		defaultMode = mode
	}

	if defaultMode == "tablet" {
		// Set tabletModeAngle to 0 to force the DUT into tablet mode.
		testing.ContextLog(ctx, "Put DUT into tablet mode")
		if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", "0", "0").Run(); err != nil {
			s.Fatal("Failed to set DUT into tablet mode: ", err)
		}
	}

	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			if err := pwrOnDut(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to login to power on DUT: ", err)
			}
		}
		if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", string(initLidAngle), string(initHys)).Run(); err != nil {
			s.Fatal("Failed to restore tablet_mode_angle to the original settings: ", err)
		}
	}(ctxForCleanUp)

	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}

	for i := 1; i <= defaultIter; i++ {
		s.Logf("Iteration: %d / %d", i, defaultIter)
		if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress); err != nil {
			s.Fatal("Failed to power long press: ", err)
		}

		sdCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		if err := dut.WaitUnreachable(sdCtx); err != nil {
			s.Fatal("Failed to shutdown DUT within 8 sec: error: ", err)
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			pwrState, err := pxy.Servo().GetECSystemPowerState(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get power state G3 error")
			}
			if pwrState != "G3" {
				return errors.New("System is not in G3")
			}
			return nil
		}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
			s.Fatal("DUT failed to enter G3 state: ", err)
		}

		if err := pwrOnDut(ctx, pxy, dut); err != nil {
			s.Fatal("Failed to login to power on DUT: ", err)
		}

		if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
			s.Fatal("Failed to login to chrome: ", err)
		}

		if err := powercontrol.ValidatePrevSleepState(ctx, dut, 5); err != nil {
			s.Fatal("Previous Sleep state is not 5: ", err)
		}
	}
}

// pwrOnDut power on DUT after pwr button long press.
func pwrOnDut(ctx context.Context, pxy *servo.Proxy, d *dut.DUT) error {
	if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Wrap(err, "failed to power normal press")
	}
	wtCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := d.WaitConnect(wtCtx); err != nil {
		if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			return errors.Wrap(err, "failed to power normal press")
		}
		if err := d.WaitConnect(wtCtx); err != nil {
			return errors.Wrap(err, "failed to wait connect DUT")
		}
	}
	return nil
}
