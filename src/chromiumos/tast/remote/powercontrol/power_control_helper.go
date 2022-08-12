// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package powercontrol

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// ChromeOSLogin performs login to DUT.
func ChromeOSLogin(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint) error {
	cl, err := rpc.Dial(ctx, dut, rpcHint)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	return nil
}

// ValidatePrevSleepState sleep state from cbmem command output.
func ValidatePrevSleepState(ctx context.Context, dut *dut.DUT, sleepStateValue int) error {
	// Command to check previous sleep state.
	const cmd = "cbmem -c | grep 'prev_sleep_state' | tail -1"
	out, err := dut.Conn().CommandContext(ctx, "sh", "-c", cmd).Output()
	if err != nil {
		return errors.Wrapf(err, "failed to execute %q command", cmd)
	}

	got := strings.TrimSpace(string(out))
	want := fmt.Sprintf("prev_sleep_state %d", sleepStateValue)

	if !strings.Contains(got, want) {
		return errors.Errorf("unexpected sleep state = got %q, want %q", got, want)
	}
	return nil
}

// ShutdownAndWaitForPowerState verifies powerState(S5 or G3) after shutdown.
func ShutdownAndWaitForPowerState(ctx context.Context, pxy *servo.Proxy, dut *dut.DUT, powerState string) error {
	powerOffCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := dut.Conn().CommandContext(powerOffCtx, "shutdown", "-h", "now").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return errors.Wrap(err, "failed to execute shutdown command")
	}
	sdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		return errors.Wrap(err, "failed to wait for unreachable")
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		got, err := pxy.Servo().GetECSystemPowerState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get EC power state")
		}
		if want := powerState; got != want {
			return errors.Errorf("unexpected DUT EC power state = got %q, want %q", got, want)
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second})
}

// WaitForSuspendState verifies powerState(S0ix or S3).
func WaitForSuspendState(ctx context.Context, h *firmware.Helper) error {
	testing.ContextLog(ctx, "Wait for power state to become S0ix or S3")
	return testing.Poll(ctx, func(ctx context.Context) error {
		state, err := h.Servo.GetECSystemPowerState(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get power state"))
		}
		if state != "S0ix" && state != "S3" {
			return errors.New("power state is " + state)
		}
		return nil
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 30 * time.Second})
}

// PowerOntoDUT performs power normal press to wake DUT.
func PowerOntoDUT(ctx context.Context, pxy *servo.Proxy, dut *dut.DUT) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			return errors.Wrap(err, "failed to power normal press")
		}
		if err := dut.WaitConnect(waitCtx); err != nil {
			return errors.Wrap(err, "failed to wait connect DUT")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Minute})
}

// PowerOnDutWithRetry performs power normal press to wake DUT. Retries if it fails.
func PowerOnDutWithRetry(ctx context.Context, pxy *servo.Proxy, dut *dut.DUT) error {
	if err := PowerOntoDUT(ctx, pxy, dut); err != nil {
		testing.ContextLog(ctx, "Unable to wake up DUT. Retrying")
		return PowerOntoDUT(ctx, pxy, dut)
	}
	return nil
}

// PerformSuspendStressTest performs suspend stress test for suspendStressTestCounter cycles.
func PerformSuspendStressTest(ctx context.Context, dut *dut.DUT, suspendStressTestCounter int) error {
	const (
		zeroPrematureWakes    = "Premature wakes: 0"
		zeroSuspendFailures   = "Suspend failures: 0"
		zeroFirmwareLogErrors = "Firmware log errors: 0"
		zeroS0ixErrors        = "s0ix errors: 0"
	)

	testing.ContextLog(ctx, "Wait for a suspend test without failures")
	zeroSuspendErrors := []string{zeroPrematureWakes, zeroSuspendFailures, zeroFirmwareLogErrors, zeroS0ixErrors}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		stressOut, err := dut.Conn().CommandContext(ctx, "suspend_stress_test", "-c", "1").Output()
		if err != nil {
			return errors.Wrap(err, "failed to execute suspend_stress_test -c 1 command")
		}

		for _, errMsg := range zeroSuspendErrors {
			if !strings.Contains(string(stressOut), errMsg) {
				return errors.Errorf("expect zero failures for %q, got %q", errMsg, string(stressOut))
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		return err
	}

	testing.ContextLogf(ctx, "Run: suspend_stress_test -c %d", suspendStressTestCounter)
	counterValue := fmt.Sprintf("%d", suspendStressTestCounter)
	stressOut, err := dut.Conn().CommandContext(ctx, "suspend_stress_test", "-c", counterValue).Output()
	if err != nil {
		return errors.Wrap(err, "failed to execute suspend_stress_test -c 10 command")
	}

	for _, errMsg := range zeroSuspendErrors {
		if !strings.Contains(string(stressOut), errMsg) {
			return errors.Errorf("failed: expect zero failures for %q, got %q", errMsg, string(stressOut))
		}
	}
	return nil
}

// SlpAndC10PackageValues returns SLP counter value and C10 package value.
func SlpAndC10PackageValues(ctx context.Context, dut *dut.DUT) (int, string, error) {
	var c10PackageRe = regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
	const (
		slpS0File     = "/sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		pkgCstateFile = "/sys/kernel/debug/pmc_core/package_cstate_show"
	)

	slpOpSetInBytes, err := linuxssh.ReadFile(ctx, dut.Conn(), slpS0File)
	if err != nil {
		return 0, "", errors.Wrap(err, "failed to get SLP counter value")
	}

	slpOpSetValue, err := strconv.Atoi(strings.TrimSpace(string(slpOpSetInBytes)))
	if err != nil {
		return 0, "", errors.Wrap(err, "failed to convert type string to integer")
	}

	pkgOpSetOutput, err := linuxssh.ReadFile(ctx, dut.Conn(), pkgCstateFile)
	if err != nil {
		return 0, "", errors.Wrap(err, "failed to get package cstate value")
	}

	matchSetValue := c10PackageRe.FindStringSubmatch(string(pkgOpSetOutput))
	if matchSetValue == nil {
		return 0, "", errors.New("failed to match pre PkgCstate value")
	}
	pkgOpSetValue := matchSetValue[1]

	return slpOpSetValue, pkgOpSetValue, nil
}

// ValidateG3PowerState verify power state G3 after shutdown.
func ValidateG3PowerState(ctx context.Context, pxy *servo.Proxy) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		pwrState, err := pxy.Servo().GetECSystemPowerState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get ec power state")
		}
		if pwrState != "G3" {
			return errors.New("DUT not in G3 state")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}

// VerifyPowerdConfigSuspendValue verifies whether DUT is in expected suspend state
// with given expectedConfigValue.
func VerifyPowerdConfigSuspendValue(ctx context.Context, dut *dut.DUT, expectedConfigValue int) error {
	powerdConfigCmd := "check_powerd_config --suspend_to_idle; echo $?"
	configValue, err := dut.Conn().CommandContext(ctx, "bash", "-c", powerdConfigCmd).Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to execute power config check command")
	}
	got, err := strconv.Atoi(strings.TrimSpace(string(configValue)))
	if err != nil {
		return errors.Wrap(err, "failed to convert string to integer")
	}
	if want := expectedConfigValue; got != want {
		return errors.Errorf("unexpected suspend state value: got %d, want %d", got, want)
	}
	return nil
}
