// Copyright 2022 The ChromiumOS Authors
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
		Attr:         []string{"group:firmware", "firmware_unstable", "firmware_ccd"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Fixture:      fixture.NormalMode,
		Timeout:      30 * time.Minute,
	})
}

const (
	fullBatteryPercent = 95

	// Battery states.
	fullyDischarged = 0x10 // (1 << 4)
	fullyCharged    = 0x20 // (1 << 5)
	discharging     = 0x40 // (1 << 6)
	initializing    = 0x80 // (1 << 7)

	// Battery alarms.
	remainingTimeAlarm      = 0x100  // (1 << 8)
	remainingCapacityAlarm  = 0x200  // (1 << 9)
	terminateDischargeAlarm = 0x800  // (1 << 11)
	overTempAlarm           = 0x1000 // (1 << 12)
	terminateChargeAlarm    = 0x4000 // (1 << 14)
	overChargedAlarm        = 0x8000 // (1 << 15)

	// terminate charge and over charged alarms are expected behavior.
	alarmMask = (0xFF00 & ^terminateChargeAlarm & ^overChargedAlarm)
)

var statusToName = map[int]string{
	fullyDischarged:         "Fully Discharged",
	fullyCharged:            "Fully Charged",
	discharging:             "Discharging",
	initializing:            "Initializing",
	remainingTimeAlarm:      "Remaining Time Alarm",
	remainingCapacityAlarm:  "Remaining Capacity Alarm",
	terminateDischargeAlarm: "Terminate Discharge Alarm",
	overTempAlarm:           "Over Temp Alarm",
	terminateChargeAlarm:    "Terminate Charge Alarm",
	overChargedAlarm:        "Over Charged Alarm",
	alarmMask:               "Alarm Mask",
}

var alarmToStatus = map[string]int{
	"EMPTY": fullyDischarged,
	"FULL":  fullyCharged,
	"DCHG":  discharging,
	"INIT":  initializing,
	"RT":    remainingTimeAlarm,
	"RC":    remainingCapacityAlarm,
	"TD":    terminateDischargeAlarm,
	"OT":    overTempAlarm,
	"TC":    terminateChargeAlarm,
	"OC":    overChargedAlarm,
}

type ecBatteryState struct {
	statusCode int64
	status     []string
	alarms     []string
	params     int64
	charging   string
	charge     int64
}

func ECChargingState(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if out, err := h.Servo.RunECCommandGetOutput(ctx, "dsleep", []string{`timeout:\s+(\d+)\s*sec`}); err == nil {
		setLowPowerIdleDelay := out[0][1]
		s.Log("Original dlseep timeout: ", setLowPowerIdleDelay)
		s.Log("Setting dsleep to 20s")
		if err := h.Servo.RunECCommand(ctx, "dsleep 20"); err != nil {
			s.Fatal("Failed to set dlseep to 20: ", err)
		}
		defer func() {
			s.Log("Setting dsleep back to original value of ", setLowPowerIdleDelay)
			if err := h.Servo.RunECCommand(ctx, "dsleep "+setLowPowerIdleDelay); err != nil {
				s.Fatalf("Failed to set dlseep to %s: %v", setLowPowerIdleDelay, err)
			}
		}()
	} else {
		s.Log("Unable to set dsleep")
	}

	s.Log("Set servo role to source to make sure suspendDUTAndCheckCharger switches")
	if err := h.SetDUTPower(ctx, true); err != nil {
		s.Fatal("Failed to set servo role to source: ", err)
	}
	s.Log("Sleeping for 3s to let servo role get set")
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		s.Fatal("Failed sleeping for 3 seconds")
	}

	if err := suspendDUTAndCheckCharger(ctx, h, false); err != nil {
		s.Fatal("Failed to suspend DUT and check if charger is attached: ", err)
	}

	battery, err := getECBatteryStatus(ctx, h)
	if err != nil {
		s.Fatal("Failed to get current battery status: ", err)
	}

	if battery.statusCode&discharging == 0 {
		s.Fatal("Incorrect battery state, expected discharging, got status: ", battery.status)
	}

	if err := compareHostAndECBatteryStatus(ctx, h, battery); err != nil {
		s.Fatal("Host and EC battery state mismatch: ", err)
	}

	if err := suspendDUTAndCheckCharger(ctx, h, true); err != nil {
		s.Fatal("Failed to suspend DUT and check if charger is attached: ", err)
	}

	battery, err = getECBatteryStatus(ctx, h)
	if err != nil {
		s.Fatal("Failed to get current battery status: ", err)
	}

	// Don't check for discharging state, fully charged occasionally also sets discharging state.
	if battery.statusCode&fullyCharged == 0 && battery.charging != "Not Allowed" && battery.statusCode&discharging != 0 {
		s.Fatal("Incorrect battery state, expected Charging/Fully Charged, got status: ", battery.status)
	}

	if err := compareHostAndECBatteryStatus(ctx, h, battery); err != nil {
		s.Fatal("Host and EC battery state mismatch: ", err)
	}

	fullChargeTimeout := 20 * time.Minute
	fullChargeInterval := 1 * time.Minute

	s.Logf("Wait for DUT to reach fully charged state up to %s minutes", fullChargeTimeout)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		battery, err = getECBatteryStatus(ctx, h)
		if err != nil {
			return errors.Wrap(err, "error getting battery status")
		} else if battery.statusCode&fullyCharged == 0 {
			return errors.Errorf("expected DUT to be fully charged, actual charge level was: %d, and status was: %v", battery.charge, battery.status)
		}
		return nil
	}, &testing.PollOptions{Timeout: fullChargeTimeout, Interval: fullChargeInterval}); err != nil {
		s.Fatal("Failed to poll for fully charged battery level in DUT")
	}
}

func compareHostAndECBatteryStatus(ctx context.Context, h *firmware.Helper, ecBatt *ecBatteryState) error {
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

	switch strings.ToLower(strings.TrimSpace(string(match[1]))) {
	case "fully charged":
		if ecBatt.statusCode&fullyCharged == 0 && ecBatt.charge < fullBatteryPercent {
			return errors.Errorf("Kernel reports battery status to be fully charged, but actual status was %v instead", ecBatt.status)
		}
		return nil
	case "charging":
		if ecBatt.statusCode&discharging != 0 {
			return errors.Errorf("Kernel reports battery status to be charging, but actual status was %v instead", ecBatt.status)
		}
		return nil
	case "not charging", "discharging":
		if ecBatt.statusCode&discharging == 0 {
			return errors.Errorf("Kernel reports battery status to be discharging, but actual status was %v instead", ecBatt.status)
		}
		return nil
	}
	return nil
}

func getECBatteryStatus(ctx context.Context, h *firmware.Helper) (*ecBatteryState, error) {
	// Example ec battery output:
	// Status:    0x00e7 FULL DCHG INIT
	// Param flags:00000002
	// Charging:  Not Allowed
	// Charge:    100 %                    (continues...)

	reStatus := `Status:\s*0[xX]([0-9a-fA-F]+)\s+((?:(?:EMPTY|FULL|DCHG|INIT)\s+)*)((?:(?:RT|RC|\--|TD|OT|TC|OC)\s+)*)[\r\n]`
	reParam := `Param\s*flags:\s*([0-9a-fA-F]+)`
	reCharging := `Charging:\s*(Allowed|Not Allowed)`
	reCharge := `Charge:\s*(\d+)`

	// batteryPattern := []string{`Status:\s*0x([0-9a-f]+).*Param flags:\s*([0-9a-f]+).*Charge:\s+(\d+)\s+`}
	batteryPattern := []string{reStatus, reParam, reCharging, reCharge}
	testing.ContextLog(ctx, "Get battery status reported by ec")

	if err := h.Servo.RunECCommand(ctx, "chan save"); err != nil {
		return nil, errors.Wrap(err, "failed to send 'chan save' to EC")
	}

	if err := h.Servo.RunECCommand(ctx, "chan 0"); err != nil {
		return nil, errors.Wrap(err, "failed to send 'chan 0' to EC")
	}

	var statusCode, params, charge int64
	status := make([]string, 0)
	alarms := make([]string, 0)
	var charging string

	// Get battery info from EC, retry in case output is corrupted/interrupted.
	out, err := h.Servo.RunECCommandGetOutput(ctx, "battery 2", batteryPattern)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run 'battery' command in ec console")
	}

	statusCode, err = strconv.ParseInt(out[0][1], 16, 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse battery status as int")
	}
	status = append(status, strings.Split(strings.TrimSpace(out[0][2]), " ")...)
	for i, v := range status {
		status[i] = statusToName[alarmToStatus[v]]
	}
	alarms = append(alarms, strings.Split(strings.TrimSpace(out[0][3]), " ")...)
	for i, v := range alarms {
		alarms[i] = statusToName[alarmToStatus[v]]
	}
	params, err = strconv.ParseInt(out[1][1], 16, 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse battery params as int")
	}
	charging = out[2][1]
	charge, err = strconv.ParseInt(out[3][1], 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse battery charge as int")
	}

	if err := h.Servo.RunECCommand(ctx, "chan 0xffffffff"); err != nil {
		return nil, errors.Wrap(err, "failed to send 'chan 0xffffffff' to EC")
	}

	if err := h.Servo.RunECCommand(ctx, "chan restore"); err != nil {
		return nil, errors.Wrap(err, "failed to send 'chan restore' to EC")
	}

	testing.ContextLog(ctx, "Verify battery does not throw any unexpected alarms")
	if statusCode&alarmMask != 0 {
		return nil, errors.Errorf("Battery threw unexpected alarms %v, had statuses: %v", alarms, status)
	}

	if (statusCode & (terminateChargeAlarm | fullyCharged)) == terminateChargeAlarm {
		return nil, errors.Errorf("Battery raising terminate charge alarm non-full, status: %v", status)
	}

	state := &ecBatteryState{
		statusCode: statusCode,
		status:     status,
		alarms:     alarms,
		params:     params,
		charging:   charging,
		charge:     charge,
	}

	return state, nil
}

func suspendDUTAndCheckCharger(ctx context.Context, h *firmware.Helper, expectChargerAttached bool) error {
	// ecSuspendDelay := 3 * time.Second
	testing.ContextLog(ctx, "Suspending DUT")
	cmd := h.DUT.Conn().CommandContext(ctx, "powerd_dbus_suspend", "--delay=3")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to suspend DUT")
	}

	testing.ContextLog(ctx, "Sleeping for 5s waiting for suspend command")
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Checking for S0ix or S3 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0ix", "S3"); err != nil {
		return errors.Wrap(err, "failed to get S0ix or S3 powerstate")
	}

	srcOrSnk := "source"
	if !expectChargerAttached {
		srcOrSnk = "sink"
	}

	testing.ContextLogf(ctx, "Set servo role to %s", srcOrSnk)
	if err := h.SetDUTPower(ctx, expectChargerAttached); err != nil {
		return errors.Wrapf(err, "failed to set servo role to %s", srcOrSnk)
	}
	testing.ContextLog(ctx, "Sleep for 3s so servo role has time to be set")
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep for 3s")
	}

	// Verify that DUT charger is in expected state.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ok, err := h.Servo.GetChargerAttached(ctx)
		if err != nil {
			testing.ContextLog(ctx, "GetChargerAttached failed: ", err)
			return errors.Wrap(err, "error checking whether charger is attached")
		} else if ok != expectChargerAttached {
			testing.ContextLogf(ctx, "GetChargerAttached got %v, want %v", ok, expectChargerAttached)
			return errors.Errorf("expected charger attached state: %v", expectChargerAttached)
		}
		return nil
		// Metaknight takes a worst case of ~120s to notice the charger, so retry for 200s instead.
	}, &testing.PollOptions{Timeout: 200 * time.Second, Interval: 4 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to check if charger is attached")
	}

	testing.ContextLog(ctx, "Power on DUT with power key")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
		return errors.Wrap(err, "failed to press power key on DUT")
	}

	testing.ContextLog(ctx, "Wait for DUT to reach S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "DUT failed to reach S0 after power button pressed")
	}

	testing.ContextLog(ctx, "Wait for DUT to connect")
	if err := h.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to DUT")
	}

	return nil
}
