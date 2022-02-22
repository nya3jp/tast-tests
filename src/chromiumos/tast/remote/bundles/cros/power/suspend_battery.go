// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SuspendBattery,
		Desc: "Tests that the DUT suspends and resumes properly while on battery power",
		Contacts: []string{
			"robertzieba@google.com",
			"tast-users@chromium.org",
		},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Fixture:      fixture.NormalMode,
		Timeout:      3 * time.Hour, // Allow time for the battery to potentially charge up
	})
}

const (
	requiredBatteryPercent = 75
	suspendCycles          = 100
	reconnectionAttempts   = 20
	tmpPowerManagerPath    = "/tmp/power_manager"
	suspendDelaySeconds    = 3
	chargeCheckDelay       = time.Minute
)

func SuspendBattery(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	// Make sure AC power is connected
	// The original setting will be restored automatically when the test ends
	if err := h.SetDUTPower(ctx, true); err != nil {
		s.Fatal("Failed to connect AC power: ", err)
	}

	// Ensure that the DUT has enough battery power to run the test
	s.Log("Checking battery charge")
	for {
		currentMAH, err := h.Servo.GetBatteryChargeMAH(ctx)
		if err != nil {
			s.Fatal("Failed to get current battery charge mAh: ", err)
		}

		maxMAH, err := h.Servo.GetBatteryFullChargeMAH(ctx)
		if err != nil {
			s.Fatal("Failed to get battery full mAh: ", err)
		}

		batteryPct := int(100 * float32(currentMAH) / float32(maxMAH))
		if batteryPct >= requiredBatteryPercent {
			s.Logf("Current battery charge is %d%%, continuing", batteryPct)
			break
		}

		s.Logf("Current battery charge is %d%%, required %d%%, checking again in %s",
			batteryPct, requiredBatteryPercent, chargeCheckDelay.String())
		testing.Sleep(ctx, chargeCheckDelay)
	}

	// Disable Servo power to DUT
	s.Log("Disconnecting AC power")
	if err := h.SetDUTPower(ctx, false); err != nil {
		s.Fatal("Failed to disconnect AC power: ", err)
	}

	type suspendType struct {
		name          string
		suspendToIdle bool
	}

	var suspendTests = []suspendType{}

	// Determine if S0ix is supported
	ret, err := runWithExitStatus(ctx, h, "grep", "-q", "freeze", "/sys/power/state")
	if err != nil {
		s.Fatal("Failed to determine S0ix support: ", err)
	}
	if ret == 0 {
		s.Log("S0ix supported")
		suspendTests = append(suspendTests, suspendType{"S0ix", true})
	}

	// Determine if S3 is supported
	ret, err = runWithExitStatus(ctx, h, "grep", "-q", "deep", "/sys/power/mem_sleep")
	if err != nil {
		s.Fatal("Failed to determine S3 support: ", err)
	}
	if ret == 0 {
		s.Log("S3 supported")
		suspendTests = append(suspendTests, suspendType{"S3", false})
	}

	// Setup powerd settings
	err = h.DUT.Conn().CommandContext(ctx, "sh", "-c", fmt.Sprintf("mkdir -p %s && "+
		"echo 0 > %s/suspend_to_idle && "+
		"mount --bind %s /var/lib/power_manager && "+
		"restart powerd",
		tmpPowerManagerPath, tmpPowerManagerPath, tmpPowerManagerPath)).Run()
	if err != nil {
		s.Fatal("Failed to setup powerd settings: ", err)
	}

	// Restore powerd settings
	defer func(ctx context.Context) {
		err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", "umount /var/lib/power_manager && restart powerd").Run()
		if err != nil {
			s.Log("Failed to restore powerd settings: ", err)
		}
	}(ctx)

	for _, test := range suspendTests {
		// Change the suspend type
		if err := setSuspendToIdle(ctx, h, test.suspendToIdle); err != nil {
			s.Fatalf("Failed to change suspend_to_idle value for %s: %s", test.name, err)
		}

		// Run our cycles
		for i := 0; i < suspendCycles; i++ {
			s.Logf("Suspend cycling %s: %d/%d", test.name, i+1, suspendCycles)
			previousCount, err := getKernelSuspendCount(ctx, h)
			if err != nil {
				s.Fatal("Failed to get kernel suspend count: ", err)
			}

			s.Log("Suspending DUT")
			cmd := h.DUT.Conn().CommandContext(ctx, "powerd_dbus_suspend", fmt.Sprintf("--delay=%d", suspendDelaySeconds))
			if err := cmd.Start(); err != nil {
				s.Fatal("Failed to suspend DUT: ", err)
			}

			testing.Sleep(ctx, suspendDelaySeconds*time.Second)
			if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, test.name); err != nil {
				s.Fatal("Failed to suspend DUT: ", err)
			}

			s.Logf("DUT Suspended to %s", test.name)
			s.Log("Power on DUT with power key")
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				s.Fatal("Failed to press power key on DUT: ", err)
			}

			s.Log("Waiting for DUT to reach S0")
			if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
				s.Fatal("DUT failed to reach S0 after power button pressed: ", err)
			}
			s.Log("DUT reached S0")

			s.Log("Waiting for DUT to reconnect")
			for i := 0; i < reconnectionAttempts && !h.DUT.Connected(ctx); i++ {
				// Take some time for the connection to recover
				testing.Sleep(ctx, time.Second)
			}

			if !h.DUT.Connected(ctx) {
				s.Fatal("Failed to reconnect to DUT after entering S0")
			}

			// Check that the kernel registered one suspension
			suspendCount, err := getKernelSuspendCount(ctx, h)
			if err != nil {
				s.Fatal("Failed to get kernel suspend count: ", err)
			}
			if suspendCount != previousCount+1 {
				s.Fatalf("Mismatch in kernel suspend counts, previous: %d, current: %d", previousCount, suspendCount)
			}
		}
	}
}

func setSuspendToIdle(ctx context.Context, h *firmware.Helper, value bool) error {
	idleValue := "0"
	if value {
		idleValue = "1"
	}

	return h.DUT.Conn().CommandContext(ctx, "sh", "-c", fmt.Sprintf("echo %s > %s/suspend_to_idle",
		idleValue, tmpPowerManagerPath)).Run()
}

func runWithExitStatus(ctx context.Context, h *firmware.Helper, name string, args ...string) (int, error) {
	err := h.DUT.Conn().CommandContext(ctx, name, args...).Run()
	if err == nil {
		// No error so we the command executed with exit code 0
		return 0, nil
	}

	if exitError := err.(*ssh.ExitError); exitError != nil {
		return exitError.ExitStatus(), nil
	}

	return -1, err
}

func getKernelSuspendCount(ctx context.Context, h *firmware.Helper) (int, error) {
	resultBytes, err := h.DUT.Conn().CommandContext(ctx, "cat", "/sys/power/suspend_stats/success").Output()
	if err != nil {
		return -1, err
	}

	return strconv.Atoi(strings.TrimSpace(string(resultBytes)))
}
