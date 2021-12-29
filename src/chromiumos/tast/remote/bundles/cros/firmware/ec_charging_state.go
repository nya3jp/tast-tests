// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECChargingState,
		Desc:         "Verify enabling and disabling write protect works as expected",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Fixture:      fixture.NormalMode,
		Timeout:      20 * time.Minute,
	})
}

const (
	fullBatteryPercent         = 95
	statusFullyCharged         = 0x20
	statusDischarging          = 0x40
	statusTerminateChargeAlarm = 0x4000
	statusAlarmMask            = (0xFF00 & ^statusTerminateChargeAlarm)
)

func ECChargingState(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if isV4, err := h.Servo.IsServoV4(ctx); err != nil {
		s.Fatal("Failed to check if servo is v4: ", err)
	} else if !isV4 {
		s.Fatal("Test requires servo v4")
	}

	if err := suspendDUT(ctx, h); err != nil {
		s.Fatal("Failed to suspend DUT")
	}

	// Verify that DUT is charging with power supply connected.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ok, err := h.Servo.GetChargerAttached(ctx)
		if err != nil {
			return errors.Wrap(err, "error checking whether charger is attached")
		} else if !ok {
			return errors.New("DUT is not charging after connecting the power supply")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}); err != nil {
		s.Fatal("Failed to check for charger after connecting power: ", err)
	}

	// PD Role gets reset by fixture after test completes.
	s.Log("Set servo role to sink")
	if err := h.SetDUTPower(ctx, false); err != nil {
		s.Fatal("Failed to set servo role to sink: ", err)
	}

	s.Log("Sleep for 3s while AC state updates")
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		s.Fatal("Failed to sleep for 3s: ", err)
	}

	s.Log("Verify AC is no longer plugged in")
	if h.Servo.ACIsPluggedIn(ctx) {
		s.Fatal("Expected AC to not be plugged in anymore but was")
	}

	s.Log("Power on DUT with power key")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		s.Fatal("Failed to press power key on DUT: ", err)
	}

	s.Log("Wait for DUT to reach S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		s.Fatal("DUT failed to reach S0 after power button pressed: ", err)
	}

	battery, err := getECBatteryStatus(ctx, h)
	if err != nil {
		s.Fatal("Failed to get current battery status: ", err)
	}

	if battery["status"]&statusDischarging == 0 {
		s.Fatal("Incorrect battery state, expected Discharging, got status: ", battery["status"])
	}

	if err := compareHostAndECBatteryStatus(ctx, h, battery); err != nil {
		s.Fatal("Host and EC battery state mismatch: ", err)
	}

	if err := suspendDUT(ctx, h); err != nil {
		s.Fatal("Failed to suspend DUT")
	}

	s.Log("Set servo role to source")
	if err := h.SetDUTPower(ctx, true); err != nil {
		s.Fatal("Failed to set servo role to source: ", err)
	}

	s.Log("Sleep for 3s while AC state updates")
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		s.Fatal("Failed to sleep for 3s: ", err)
	}

	s.Log("Verify AC is plugged in")
	if !h.Servo.ACIsPluggedIn(ctx) {
		s.Fatal("Expected AC to not be plugged in anymore but was")
	}

	s.Log("Power on DUT with power key")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		s.Fatal("Failed to press power key on DUT: ", err)
	}

	s.Log("Wait for DUT to reach S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		s.Fatal("DUT failed to reach S0 after power button pressed: ", err)
	}

	battery, err = getECBatteryStatus(ctx, h)
	if err != nil {
		s.Fatal("Failed to get current battery status: ", err)
	}

	if battery["status"]&statusFullyCharged == 0 && battery["status"]&statusDischarging != 0 {
		s.Fatal("Incorrect battery state, expected Charging/Fully Charged, got status: ", battery["status"])
	}

	if err := compareHostAndECBatteryStatus(ctx, h, battery); err != nil {
		s.Fatal("Host and EC battery state mismatch: ", err)
	}

	fullChargeDeadline := 15 * time.Minute
	fullChargeInterval := 1 * time.Minute

	s.Logf("Wait for DUT to reach fully charged state up to %s minutes", fullChargeDeadline)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		battery, err = getECBatteryStatus(ctx, h)
		if err != nil {
			return errors.Wrap(err, "error getting battery status")
		} else if battery["status"]&statusFullyCharged == 0 {
			return errors.Errorf("expected DUT to be fully charged, actual charge level was: %d", battery["level"])
		}
		return nil
	}, &testing.PollOptions{Timeout: fullChargeInterval, Interval: fullChargeDeadline}); err != nil {
		s.Fatal("Failed to poll for fully charged battery level in DUT")
	}
}

func compareHostAndECBatteryStatus(ctx context.Context, h *firmware.Helper, ecBatt map[string]int64) error {
	testing.ContextLog(ctx, "Get battery status reported by kernel")
	out, err := h.DUT.Conn().CommandContext(ctx, "power_supply_info").Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to get power supply info")
	}

	// Matches the state property under the device battery.
	reBattState := regexp.MustCompile(`(?s)Device: Battery.*state:(?-s)\s+(\S+(\s\S+)*)\s`)
	match := reBattState.FindSubmatch(out)
	if match == nil {
		return errors.Errorf("failed to parse battery state in output: %s", string(out))
	}
	testing.ContextLog(ctx, "Kernel reported battery status: ", string(match[1]))

	switch strings.ToLower(string(match[1])) {
	case "fully charged":
		if ecBatt["status"]&statusFullyCharged == 0 && ecBatt["level"] < fullBatteryPercent {
			return errors.Errorf("Kernel reports battery status to be fully charged, but actual state was %v instead", ecBatt["status"])
		}
		return nil
	case "charging":
		if ecBatt["status"]&statusDischarging != 0 {
			return errors.Errorf("Kernel reports battery status to be charging, but actual state was %v instead", ecBatt["status"])
		}
		return nil
	case "not charging", "discharging":
		if ecBatt["status"]&statusDischarging == 0 {
			return errors.Errorf("Kernel reports battery status to be discharging, but actual state was %v instead", ecBatt["status"])
		}
		return nil
	}
	return nil
}

func getECBatteryStatus(ctx context.Context, h *firmware.Helper) (map[string]int64, error) {
	batteryPattern := []string{`Status:\s*0x([0-9a-f]+).*Param flags:\s*([0-9a-f]+).*Charge:\s+(\d+)\s+`}
	testing.ContextLog(ctx, "Get battery status reported by ec")
	out, err := h.Servo.RunECCommandGetOutput(ctx, "battery", batteryPattern)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get battery info")
	}
	status, err := strconv.ParseInt(out[0][1], 16, 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse battery status as int")
	}
	params, err := strconv.ParseInt(out[0][2], 16, 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse battery params as int")
	}
	charge, err := strconv.ParseInt(out[0][3], 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse battery charge as int")
	}

	testing.ContextLog(ctx, "Verify battery does not throw any unexpected alarms")
	if status&statusAlarmMask != 0 {
		return nil, errors.Errorf("Battery should not throw alarms, status: %v", status)
	}

	if (status & (statusTerminateChargeAlarm | statusFullyCharged)) == statusTerminateChargeAlarm {
		return nil, errors.Errorf("Battery raising terminate charge alarm non-full, status: %v", status)
	}

	return map[string]int64{
		"status": status,
		"params": params,
		"charge": charge,
	}, nil
}

func suspendDUT(ctx context.Context, h *firmware.Helper) error {
	ecSuspendDelay := 3 * time.Second
	testing.ContextLog(ctx, "Suspending DUT")
	cmd := h.DUT.Conn().CommandContext(ctx, "powerd_dbus_suspend")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to suspend DUT")
	}

	testing.ContextLogf(ctx, "Sleeping for %s", ecSuspendDelay)
	if err := testing.Sleep(ctx, ecSuspendDelay); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Checking for S0ix or S3 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0ix", "S3"); err != nil {
		return errors.Wrap(err, "failed to get S0ix or S3 powerstate")
	}
	return nil
}
