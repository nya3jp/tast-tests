// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerformColdRebootDuringS0ix,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies cold reboot functionality during sleep",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.X86()),
		Timeout:      7 * time.Minute,
	})
}

func PerformColdRebootDuringS0ix(ctx context.Context, s *testing.State) {
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
		s.Log("Performing cleanup")
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Error("Failed to power-on DUT at cleanup: ", err)
			}
		}
		if err := dut.Conn().CommandContext(ctx, "sh", "-c", "umount /var/lib/power_manager && restart powerd").Run(ssh.DumpLogOnError); err != nil {
			s.Log("Failed to restore powerd settings: ", err)
		}
	}(cleanupCtx)

	// Performs Chrome login.
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login Chrome: ", err)
	}

	if err := dut.Conn().CommandContext(ctx, "sh", "-c", fmt.Sprintf(
		"mkdir -p /tmp/power_manager && "+
			"echo 1 > /tmp/power_manager/suspend_to_idle && "+
			"mount --bind /tmp/power_manager /var/lib/power_manager && "+
			"restart powerd"),
	).Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to set suspend to idle: ", err)
	}

	const expectedS0ixConfigValue = 0
	if err := powercontrol.VerifyPowerdConfigSuspendValue(ctx, dut, expectedS0ixConfigValue); err != nil {
		s.Fatal("Failed to verfiy power config value for S0ix: ", err)
	}

	// As soon as Chrome login, if suspend command is executed
	// DUT fails to go to suspend/unreachable state.
	// So, short sleep of 1s is expected before suspending DUT.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err := powercontrol.PerformPowerdbusSuspend(ctx, dut); err != nil {
		s.Fatal("Failed to perform powerdbus suspend: ", err)
	}

	firmwareHelper := &firmware.Helper{Servo: pxy.Servo()}
	if err := powercontrol.WaitForSuspendState(ctx, firmwareHelper); err != nil {
		s.Fatal("Failed to wait for DUT suspend state: ", err)
	}

	// In suspend state cold reboot DUT with EC console 'reboot' command.
	s.Log("Reboot DUT during sleep state")
	if _, err := pxy.Servo().RunECCommandGetOutput(ctx, "reboot", []string{`Rebooting!`}); err != nil {
		s.Fatal("Failed to execute 'reboot' command on EC console: ", err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := dut.WaitConnect(waitCtx); err != nil {
		s.Fatal("Failed to wait connect DUT after reboot: ", err)
	}

	// cbmemSleepState value must be 5 for cold reboot.
	cbmemSleepState := 5
	if err := powercontrol.ValidatePrevSleepState(ctx, dut, cbmemSleepState); err != nil {
		s.Fatal("Failed to validate previous sleep state: ", err)
	}

	// Again performing powerdbus suspend.
	if err := powercontrol.PerformPowerdbusSuspend(ctx, dut); err != nil {
		s.Fatal("Failed to perform powerdbus suspend: ", err)
	}

	if err := powercontrol.WaitForSuspendState(ctx, firmwareHelper); err != nil {
		s.Fatal("Failed to wait for DUT suspend state: ", err)
	}

	// In suspend state cold reboot DUT with refresh+power button press via servo.
	s.Log("Pressing refresh + power key to boot up DUT")
	if err := pxy.Servo().KeypressWithDuration(ctx, servo.Refresh, servo.DurLongPress); err != nil {
		s.Fatal("Failed to press refresh key: ", err)
	}
	if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		s.Fatal("Failed to power normal press: ", err)
	}
	waitCtx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := dut.WaitConnect(waitCtx); err != nil {
		s.Fatal("Failed to wait connect DUT with refresh+power button press: ", err)
	}

	if err := powercontrol.ValidatePrevSleepState(ctx, dut, cbmemSleepState); err != nil {
		s.Fatal("Failed to validate previous sleep state: ", err)
	}
}
