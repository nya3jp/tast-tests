// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/usbutils"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         USBDeviceHotplugDuringSleep,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies USB device functionality with hotplug during sleep",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome", "reboot"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		VarDeps:      []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.X86()),
		Timeout:      5 * time.Minute,
	})
}

func USBDeviceHotplugDuringSleep(ctx context.Context, s *testing.State) {
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

	cmdRun := func(cmd string) {
		if err := dut.Conn().CommandContext(ctx, "bash", "-c", cmd).Run(); err != nil {
			s.Fatalf("Failed to execute %s command: %v", cmd, err)
		}
	}

	const (
		enableIdleSuspendCommand  = "echo 0 > /var/lib/power_manager/disable_idle_suspend"
		disableIdleSuspendCommand = "echo 1 > /var/lib/power_manager/disable_idle_suspend"
		restartPowerdCommand      = "restart powerd"
		umountCommand             = "umount /var/lib/power_manager && restart powerd"
	)

	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power-on DUT at cleanup: ", err)
			}
		}
		cmdRun(umountCommand)
		cmdRun(disableIdleSuspendCommand)
		cmdRun(restartPowerdCommand)
	}(cleanupCtx)

	mountCommand := fmt.Sprintf(
		"mkdir -p /tmp/power_manager && " +
			"echo 1 > /tmp/power_manager/suspend_to_idle && " +
			"mount --bind /tmp/power_manager /var/lib/power_manager && " +
			"restart powerd")
	cmdRun(mountCommand)

	const expectedConfigValue = 0
	if err := powercontrol.VerifyPowerdConfigSuspendValue(ctx, dut, expectedConfigValue); err != nil {
		s.Fatal("Failed to verfiy power config value for S0ix: ", err)
	}

	cmdRun(enableIdleSuspendCommand)
	cmdRun(restartPowerdCommand)

	initialMuxState, err := pxy.Servo().GetUSBMuxState(ctx)
	if err != nil {
		s.Fatal("Failed to get USB Mux state info: ", err)
	}
	defer pxy.Servo().SetUSBMuxState(cleanupCtx, initialMuxState)

	// Perform initial Chrome login.
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to log in to Chrome: ", err)
	}

	if err := pxy.Servo().SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
		s.Fatal("Failed to plug USB storage device to DUT: ", err)
	}

	// Check for USB storage device detection.
	if err := waitForUSBStorageDetection(ctx, dut); err != nil {
		s.Fatal("Failed to detect USB storage device: ", err)
	}

	if err := pxy.Servo().SetUSBMuxState(ctx, servo.USBMuxHost); err != nil {
		s.Fatal("Failed to unplug USB storage device from DUT: ", err)
	}

	slpOpSetPre, pkgOpSetPre, err := powercontrol.SlpAndC10PackageValues(ctx, dut)
	if err != nil {
		s.Fatal("Failed to get SLP counter and C10 package values before suspend-resume: ", err)
	}

	if err := performSetPowerPolicySuspend(ctx, pxy, dut); err != nil {
		s.Fatal("Failed to perform suspend and wake DUT: ", err)
	}

	if err := pxy.Servo().SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
		s.Fatal("Failed to hotplug USB storage device to DUT during suspend: ", err)
	}

	if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
		s.Fatal("Failed to wake DUT: ", err)
	}

	if err := dut.Conn().CommandContext(ctx, "set_power_policy", "reset").Run(); err != nil {
		s.Error("Failed to reset set_power_policy: ", err)
	}

	// Check for USB storage device detection after suspend-resume.
	if err := waitForUSBStorageDetection(ctx, dut); err != nil {
		s.Fatal("Failed to detect USB storage device after suspend: ", err)
	}

	slpOpSetPost, pkgOpSetPost, err := powercontrol.SlpAndC10PackageValues(ctx, dut)
	if err != nil {
		s.Fatal("Failed to get SLP counter and C10 package values after suspend-resume: ", err)
	}

	if slpOpSetPre == slpOpSetPost {
		s.Fatalf("Failed: SLP counter value %q should be different from the one before suspend %q", slpOpSetPost, slpOpSetPre)
	}

	if slpOpSetPost == 0 {
		s.Fatal("Failed SLP counter value must be non-zero, got: ", slpOpSetPost)
	}

	if pkgOpSetPre == pkgOpSetPost {
		s.Fatalf("Failed: Package C10 value %q must be different from the one before suspend %q", pkgOpSetPost, pkgOpSetPre)
	}

	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		s.Fatal("Failed: Package C10 should be non-zero")
	}
}

// performSetPowerPolicySuspend performs DUT suspend with 'set_power_policy' command and
// wakes DUT with servo power key press.
func performSetPowerPolicySuspend(ctx context.Context, pxy *servo.Proxy, dut *dut.DUT) error {
	powerOffCtx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	if err := dut.Conn().CommandContext(powerOffCtx, "set_power_policy", "--battery_idle_delay=6", "--ac_idle_delay=6").Run(); err != nil {
		return errors.Wrap(err, "failed to power off DUT")
	}

	suspendCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(suspendCtx); err != nil {
		return errors.Wrap(err, "failed to wait for unreachable")
	}
	return nil
}

// waitForUSBStorageDetection checks for connected USB storage detection.
func waitForUSBStorageDetection(ctx context.Context, dut *dut.DUT) error {
	usbDeviceClassName := "Mass Storage"
	usbSpeed := "5000M"
	return testing.Poll(ctx, func(ctx context.Context) error {
		usbDevicesList, err := usbutils.ListDevicesInfo(ctx, dut)
		if err != nil {
			return errors.Wrap(err, "failed to get USB devices list")
		}
		got := usbutils.NumberOfUSBDevicesConnected(usbDevicesList, usbDeviceClassName, usbSpeed)
		if want := 1; got != want {
			return errors.Errorf("unexpected number of %q devices connected with %q speed: got %d, want %d",
				usbDeviceClassName, usbSpeed, got, want)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}
