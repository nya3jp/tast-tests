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
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	requiredBatteryPercent = 75
	minBatteryPercent      = 50
	defaultCycles          = 100
	defaultAllowS2idle     = true
	reconnectionTimeout    = 20 * time.Second
	reconnectionInterval   = time.Second
	tmpPowerManagerPath    = "/tmp/power_manager"
	suspendDelaySeconds    = 3
	chargeCheckInterval    = time.Minute
	chargeCheckTimeout     = time.Hour
	servoCommTimeout       = 5 * time.Second
	servoCommInterval      = time.Second

	varCycles    = "cycles"
	varAllowS0ix = "allow_s0ix"
	varAllowS3   = "allow_s3"
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
		Vars:         []string{varCycles, varAllowS0ix, varAllowS3},
	})
}

func SuspendBattery(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	// Ensure that the DUT has enough battery power to run the test
	s.Logf("Waiting for battery to reach %d%%", requiredBatteryPercent)
	if err := waitForCharge(ctx, h, requiredBatteryPercent); err != nil {
		s.Fatalf("Failed to reach target %d%%, %s", requiredBatteryPercent, err.Error())
	}

	// Parse our vars
	suspendCycles := defaultCycles
	if v, ok := s.Var(varCycles); ok {
		newCycles, err := strconv.Atoi(v)
		if err != nil {
			s.Fatalf("Failed to parse %s from string %s", varCycles, v)
		}

		suspendCycles = newCycles
	}

	allowS0ix := true
	if v, ok := s.Var(varAllowS0ix); ok {
		newAllowS0ix, err := strconv.ParseBool(v)
		if err != nil {
			s.Fatalf("Failed to parse %s from string %s", varAllowS0ix, v)
		}

		allowS0ix = newAllowS0ix
	}

	allowS3 := true
	if v, ok := s.Var(varAllowS3); ok {
		newAllowS3, err := strconv.ParseBool(v)
		if err != nil {
			s.Fatalf("Failed to parse %s from string %s", varAllowS3, v)
		}

		allowS3 = newAllowS3
	}

	// Setup powerd settings
	err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", fmt.Sprintf("mkdir -p %s && "+
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
		if allowS0ix {
			// The most reliable way to check if a sleep state is supported is to attempt to enter that state
			setSuspendToIdle(ctx, h, true)
			if err := suspendCycleDut(ctx, h, "S0ix"); err != nil {
				s.Fatalf("S0ix is supported, but failed to enter state: %s", err)
			}

			s.Log("Testing S0ix")
			suspendTests = append(suspendTests, suspendType{"S0ix", true})
		} else {
			s.Logf("S0ix testing disabled through %s", varAllowS0ix)
		}
	}

	// Determine if S3 is supported
	ret, err = runWithExitStatus(ctx, h, "grep", "-q", "deep", "/sys/power/mem_sleep")
	if err != nil {
		s.Fatal("Failed to determine S3 support: ", err)
	}
	if ret == 0 {
		if allowS3 {
			// The most reliable way to check if a sleep state is supported is to attempt to enter that state
			setSuspendToIdle(ctx, h, false)
			if err := suspendCycleDut(ctx, h, "S3"); err != nil {
				s.Fatalf("S3 is supported, but failed to enter state: %s", err)
			}

			s.Log("Testing S3")
			suspendTests = append(suspendTests, suspendType{"S3", false})
		} else {
			s.Logf("S3 testing disabled through %s", varAllowS3)
		}
	}

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
			if err := suspendCycleDut(ctx, h, test.name); err != nil {
				s.Fatal("Failed to suspend cycle DUT: ", err)
			}

			// Check that the kernel registered one suspension
			suspendCount, err := getKernelSuspendCount(ctx, h)
			if err != nil {
				s.Fatal("Failed to get kernel suspend count: ", err)
			}
			if suspendCount != previousCount+1 {
				s.Fatalf("Mismatch in kernel suspend counts, previous: %d, current: %d", previousCount, suspendCount)
			}

			//Charge up if we've dipped below our minimum battery level
			pct, err := getBatteryPercent(ctx, h)
			if err != nil {
				s.Fatal("Failed to get battery level")
			}

			if pct < minBatteryPercent {
				s.Logf("Waiting for battery to reach %d%%", requiredBatteryPercent)
				waitForCharge(ctx, h, requiredBatteryPercent)
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
	resultBytes, err := h.DUT.Conn().CommandContext(ctx, "cat", "/sys/kernel/debug/suspend_stats").Output()
	if err != nil {
		return -1, err
	}

	lines := strings.Split(string(resultBytes), "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "success") {
			continue
		}

		split := strings.Split(line, ":")
		if len(split) != 2 {
			return -1, errors.New("Improperly formatted success line")
		}

		return strconv.Atoi(strings.TrimSpace(split[1]))
	}

	return -1, errors.New("failed to find succes line")
}

func waitForCharge(ctx context.Context, h *firmware.Helper, target int) error {
	// Make sure AC power is connected
	// The original setting will be restored automatically when the test ends
	if err := h.SetDUTPower(ctx, true); err != nil {
		return err
	}

	err := testing.Poll(ctx, func(ctx context.Context) error {
		pct, err := getBatteryPercent(ctx, h)
		if err != nil {
			// Failed to get battery level so stop trying
			return testing.PollBreak(err)
		}

		if pct < target {
			return errors.Errorf("Current battery charge is %d%%, required %d%%", pct, target)
		}

		return nil
	}, &testing.PollOptions{Timeout: chargeCheckTimeout, Interval: chargeCheckInterval})

	if err != nil {
		return err
	}

	// Disable Servo power to DUT
	if err := h.SetDUTPower(ctx, false); err != nil {
		return err
	}

	return nil
}

func getBatteryPercent(ctx context.Context, h *firmware.Helper) (int, error) {
	// Attempt to determine the battery percentage
	// Each servo communication attempt is retried to account for any transient
	// communication problems
	var err error = nil
	currentMAH := 0
	maxMAH := 0

	testing.Poll(ctx, func(ctx context.Context) error {
		currentMAH, err = h.Servo.GetBatteryChargeMAH(ctx)
		if err != nil {
			return err
		}

		maxMAH, err = h.Servo.GetBatteryFullChargeMAH(ctx)
		if err != nil {
			return err
		}

		return nil

	}, &testing.PollOptions{Timeout: servoCommTimeout, Interval: servoCommInterval})

	if err != nil {
		return -1, err
	}

	return int(100 * float32(currentMAH) / float32(maxMAH)), nil
}

func suspendCycleDut(ctx context.Context, h *firmware.Helper, targetState string) error {
	cmd := h.DUT.Conn().CommandContext(ctx, "powerd_dbus_suspend", fmt.Sprintf("--delay=%d", suspendDelaySeconds))
	if err := cmd.Start(); err != nil {
		return errors.Errorf("failed to invoke powerd_dbus_suspend: %s", err)
	}

	testing.Sleep(ctx, suspendDelaySeconds*time.Second)
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, targetState); err != nil {
		return errors.Errorf("failed to get power state %s: %s", targetState, err)
	}

	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Errorf("failed to press power key on DUT: %s", err)
	}

	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Errorf("DUT failed to reach S0 after power button pressed: %s", err)
	}

	err := testing.Poll(ctx, func(ctx context.Context) error {
		if !h.DUT.Connected(ctx) {
			return errors.New("waiting for DUT to reconnect")
		}

		return nil

	}, &testing.PollOptions{Timeout: reconnectionTimeout, Interval: reconnectionInterval})

	if err != nil {
		return errors.New("failed to reconnect to DUT after entering S0")
	}

	return nil
}
