// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     FlagsPreservation,
		Desc:     "Checks that flag values are preserved over different power cycles",
		Contacts: []string{"arthur.chuang@cienet.com", "chromeos-firmware@google.com"},
		// TODO(b/235742217): This test might be leaving broken DUTS that can't be auto-repaired. Add attr firmware_unstable when fixed.
		Attr:         []string{"group:firmware"},
		Fixture:      fixture.DevModeGBB,
		SoftwareDeps: []string{"crossystem"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Timeout:      30 * time.Minute,
	})
}

type crossystemValues struct {
	devBootUSBVal      string
	devBootAltfw       string
	fwUpdateTriesVal   string
	locIdxVal          string
	backupNvramRequest string
}

func FlagsPreservation(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create config: ", err)
	}

	// Check if DUT uses vboot2.
	vboot2, err := h.Reporter.Vboot2(ctx)
	if err != nil {
		s.Fatal("Failed to determine fw_vboot2: ", err)
	}

	// For legacy devices with vboot1, reboot if crossystem backup_nvram_request
	// doesn't return 0.
	if !vboot2 {
		shouldReboot, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamBackupNvramRequest)
		if err != nil {
			s.Fatal("Failed to run crossystem param: ", err)
		}
		if shouldReboot != "0" {
			s.Logf("Got crossystem backup_nvram_request value: %s, rebooting DUT now", shouldReboot)
			if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
				s.Fatal("Failed to reboot DUT by servo: ", err)
			}
		}
	}

	cs := crossystemValues{}
	s.Log("Saving original crossystem values under evaluation to restore at the end of test")
	csOriginal, err := readTargetCsVals(ctx, s, h, cs)
	if err != nil {
		s.Fatal("Failed to read initial crossystem values: ", err)
	}
	originalCrossystemMap := createCsMap(csOriginal)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Minute)
	defer cancel()

	defer func(ctx context.Context) {
		// Run a cold reset first to ensure DUT connected.
		s.Log("Cold resetting DUT at the end of test")
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
			s.Fatal("Failed to cold reset DUT at the end of test: ", err)
		}
		s.Log("Waiting for reconnection to DUT")
		if err := h.WaitConnect(ctx); err != nil {
			s.Fatal("Unable to reconnect to DUT: ", err)
		}
		s.Log("Restoring crossystem values to the original settings")
		if err := setTargetCsVals(ctx, s, h, originalCrossystemMap); err != nil {
			s.Fatal("Failed to restore crossystem values to the original settings: ", err)
		}
	}(cleanupCtx)

	// Target crossystem params and their values to be tested.
	targetCrossystemMap := map[string]string{
		"dev_boot_usb":   "1",
		"dev_boot_altfw": "1",
		"fwupdate_tries": "2",
		"loc_idx":        "3",
	}

	s.Log("Setting crossystem params with target values")
	if err := setTargetCsVals(ctx, s, h, targetCrossystemMap); err != nil {
		s.Fatal("Failed to set target crossystem values: ", err)
	}

	for _, tc := range []struct {
		powerDisruption string
		fwVboot2        bool
	}{
		{"powerCycleByReboot", vboot2},
		{"powerCycleByPressingPowerKey", vboot2},
		{"powerCycleByRemovingBattery", vboot2},
	} {
		s.Log("Saving crossystem params and their values before a power-cycle")
		csBefore, err := readTargetCsVals(ctx, s, h, cs)
		if err != nil {
			s.Fatal("Failed to read crossystem values before a power-cycle: ", err)
		}
		beforeCrossystemMap := createCsMap(csBefore)

		// For legacy devices with vboot1, crossystem backup_nvram_request should return 1.
		if !tc.fwVboot2 {
			if csBefore.backupNvramRequest != "1" {
				s.Fatalf("DUT is a legacy device. Expected value 1 before power-cycle from crossystem backup_nvram_request, but got %s", csBefore.backupNvramRequest)
			}
		}

		switch tc.powerDisruption {
		case "powerCycleByReboot":
			s.Log("Power-cycling DUT with a reboot")
			if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
				s.Fatal("Failed to reboot DUT by servo: ", err)
			}
		case "powerCycleByPressingPowerKey":
			// pressPowerBtn sends a press on the power button and waits for DUT to reach G3.
			pressPowerBtn := func(pressDur time.Duration) error {
				s.Logf("Power-cycling DUT by pressing power button for %s", pressDur)
				if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(pressDur)); err != nil {
					return errors.Wrap(err, "failed to press power through servo")
				}
				s.Log("Waiting for power state to become G3")
				if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 2*time.Minute, "G3"); err != nil {
					return errors.Wrap(err, "failed to get powerstate at G3")
				}
				return nil
			}
			// Retry powering off DUT with a longer press if the duration was less than
			// 10 seconds on the power button, and the DUT remained at S0.
			powerOff := h.Config.HoldPwrButtonPowerOff
			if powerOff >= 10*time.Second {
				if err := pressPowerBtn(powerOff); err != nil {
					s.Fatal("Failed to power off DUT: ", err)
				}
			} else {
				for powerOff < 10*time.Second {
					err := pressPowerBtn(powerOff)
					if err == nil {
						break
					}
					powerState, _ := checkPowerState(ctx, h)
					if powerState != "S0" || powerOff == 9*time.Second {
						s.Fatal("Failed to power off DUT: ", err)
					}
					powerOff += 1 * time.Second
				}
			}
			s.Log("Pressing on the power button to power on DUT")
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				s.Fatal("Failed to perform a tap on the power button: ", err)
			}
		case "powerCycleByRemovingBattery":
			// Opening CCD prior to battery cutoff would help some DUTs in waking
			// when ac is re-attached.
			if err := openCCD(ctx, h); err != nil {
				s.Fatal("CCD not opened: ", err)
			}

			// Explicitly log servo type and dut connection type for debugging purposes.
			s.Log("Logging servo type and dut connection type")
			var validInfo = []string{"servoType", "dutConnectionType"}
			for _, info := range validInfo {
				if result, err := logInformation(ctx, h, info); err != nil {
					s.Logf("Unable to find information on %s: %v", info, err)
				} else {
					s.Logf("%s: %s", info, result)
				}
			}

			// For debugging purposes, log servo serials to check
			// whether servo v4 or v4.1 is used.
			servoSerials, err := h.Servo.GetServoSerials(ctx)
			if err != nil {
				s.Fatal("Failed to get servo serials: ", err)
			}
			for serial, val := range servoSerials {
				s.Logf("Found serial for %s: %s", serial, val)
			}
			// Dirinboz [zork] did not wake from battery cutoff with a 45W charger,
			// but it did with a 65W. Also, when connected to a servo v4 board, the
			// same 65W charger only outputs 60W to DUT. On servo v4.1, it output the
			// full 65W. Log more information about how much power DUT receives from
			// charger in the lab.
			if err := checkMaxChargerPower(ctx, h); err != nil {
				s.Fatal("Failed to check for max power: ", err)
			}

			s.Log("Power-cycling DUT by disconnecting AC and removing battery")
			if err := h.SetDUTPower(ctx, false); err != nil {
				s.Fatal("Failed to remove charger: ", err)
			}

			// When power is cut, there's a temporary drop in connection with the DUT.
			// Wait for DUT to reconnect before proceeding to the next step.
			waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
			defer cancelWaitConnect()
			if err := s.DUT().WaitConnect(waitConnectCtx); err != nil {
				s.Fatal("Failed to reconnect to DUT: ", err)
			}

			// Log information on PD communication status after disconnecting charger.
			if pdStatus, err := logInformation(ctx, h, "pdCommunication"); err != nil {
				s.Log("Unable to check PD communication status: ", err)
			} else {
				s.Logf("PD communication status: %s", pdStatus)
			}

			// Check that charger was removed.
			getChargerPollOptions := testing.PollOptions{
				Timeout:  20 * time.Second,
				Interval: 1 * time.Second,
			}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
					return err
				} else if attached {
					return errors.New("charger is still attached - use Servo V4 Type-C or supply RPM vars")
				}
				return nil
			}, &getChargerPollOptions); err != nil {
				s.Logf("Check for charger failed: %v Attempting to check DUT's battery status", err)
				status, err := h.Reporter.BatteryStatus(ctx)
				if err != nil {
					s.Fatal("Check for battery status failed: ", err)
				} else if status != "Discharging" {
					s.Fatalf("Unexpected battery status after removing charger: %s", status)
				}
				s.Logf("Battery Status: %s", status)
			}

			// Between removing charger, and sending battery cutoff,
			// waiting for some delay seems to help prevent servo exit.
			s.Log("Sleeping for 30 seconds")
			if err := testing.Sleep(ctx, 30*time.Second); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}

			s.Log("Removing CCD watchdog")
			if err := h.Servo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
				s.Fatal("Failed to remove watchdog for ccd: ", err)
			}

			// Check if DUT is offline after battery cutoff at the end of test. If it is,
			// first check if power state is in S5 or G3, and boot DUT to S0 by pressing
			// on power button. If there's no response at all, disconnecting and then
			// reconnecting charger seems to help with restoring the connection.
			defer func(ctx context.Context) {
				if !s.DUT().Connected(ctx) {
					if err := checkDUTAsleepAndPressPwr(ctx, h); err != nil {
						s.Logf("DUT completely offline: %v Attempting to restore connection", err)
						if err := h.Servo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
							s.Fatal("Failed to remove watchdog for ccd: ", err)
						}
						// To-do: depending on how the results turn out, we could
						// extend the sleep in WatchdogRemove(), instead of adding
						// another sleep here.
						s.Log("Sleeping for 5 seconds")
						if err := testing.Sleep(ctx, 5*time.Second); err != nil {
							s.Fatal("Failed to sleep: ", err)
						}
						s.Log("Removing DUT's power")
						if err := h.SetDUTPower(ctx, false); err != nil {
							s.Fatal("Failed to set pd role: ", err)
						}
						s.Log("Sleeping for 60 seconds")
						if err := testing.Sleep(ctx, 60*time.Second); err != nil {
							s.Fatal("Failed to sleep: ", err)
						}
						s.Log("Reconnecting DUT's power")
						if err := h.SetDUTPower(ctx, true); err != nil {
							s.Fatal("Failed to set pd role: ", err)
						}
					}
					if err := h.WaitConnect(ctx); err != nil {
						s.Fatal("DUT did not wake up: ", err)
					}
					if err := openCCD(ctx, h); err != nil {
						s.Fatal("CCD not opened: ", err)
					}
				}
			}(cleanupCtx)

			s.Log("Cutting off DUT's battery")
			cmd := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain)
			if err := cmd.BatteryCutoff(ctx); err != nil {
				s.Fatal("Failed to send the battery cutoff command: ", err)
			}

			// 60 seconds of sleep may be necessary in order for some batteries to
			// be fully cut off, and for reducing complications in waking DUTs.
			s.Log("Sleeping for 60 seconds")
			if err := testing.Sleep(ctx, 60*time.Second); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}

			s.Log("Checking EC unresponsive")
			if err := h.Servo.CheckUnresponsiveEC(ctx); err != nil {
				s.Fatal("While verifying whether EC is unresponsive after cutting off DUT's battery: ", err)
			}

			s.Log("Powering on DUT by reconnecting AC")
			if err := h.SetDUTPower(ctx, true); err != nil {
				s.Fatal("Failed to reconnect charger: ", err)
			}
		}

		s.Log("Waiting for DUT to power ON")
		waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 8*time.Minute)
		defer cancelWaitConnect()

		if err := h.WaitConnect(waitConnectCtx); err != nil {
			// When reconnecting to the DUT fails from plugging in AC,
			// check for its power state, charge state, and whether
			// the battery is detected (present). Also, log gpio output
			// indicating when the system power is good for AP to pwr up.
			pwrState, _ := checkPowerState(ctx, h)
			stateOfCharge, _ := checkChgstateBatt(ctx, h, "state_of_charge")
			battPresent, _ := checkChgstateBatt(ctx, h, "is_present")
			// To-do: EC_FCH_PWROK was configured for zork devices, for example,
			// vilboz and dirinboz. Add more gpio names in the future if more
			// models are to be checked.
			pwrOkGpio, _ := grepGpio(ctx, h, "EC_FCH_PWROK")
			s.Fatalf("Failed to reconnect to DUT [power state %s, battery %s, %s, gpio_pwrok %s]: %v",
				pwrState, stateOfCharge, battPresent, pwrOkGpio, err)
		}
		// Cr50 goes to sleep when the battery is disconnected, and when DUT wakes,
		// CCD might be locked. Open CCD after waking DUT and before talking to the EC.
		if err := openCCD(ctx, h); err != nil {
			s.Fatal("CCD not opened: ", err)
		}

		s.Log("Saving crossystem params and their values after a power-cycle")
		csAfter, err := readTargetCsVals(ctx, s, h, cs)
		if err != nil {
			s.Fatal("Failed to read crossystem values after a power-cycle: ", err)
		}
		afterCrossystemMap := createCsMap(csAfter)

		// For legacy devices with vboot1, crossystem backup_nvram_request should return 0.
		if !tc.fwVboot2 {
			if csAfter.backupNvramRequest != "0" {
				s.Fatalf("DUT is a legacy device. Expected value 0 after power-cycle from crossystem backup_nvram_request, but got %s", csAfter.backupNvramRequest)
			}
		}

		// Compare the before and after values in crossystemValues, except backupNvramRequest,
		// which is only checked on DUTs using vboot1.
		if ok, err := equal(ctx, beforeCrossystemMap, afterCrossystemMap); err != nil || !ok {
			s.Fatal("Crossystem values are different after power-cycle: ", err)
		} else if ok {
			s.Log("Flags are preserved after power-cycle")
		}

	}
}

func createCsMap(cs *crossystemValues) map[string]string {
	var csParamsMap = map[string]string{
		"dev_boot_usb":   cs.devBootUSBVal,
		"dev_boot_altfw": cs.devBootAltfw,
		"fwupdate_tries": cs.fwUpdateTriesVal,
		"loc_idx":        cs.locIdxVal,
	}
	return csParamsMap
}

func readTargetCsVals(ctx context.Context, s *testing.State, h *firmware.Helper, cs crossystemValues) (*crossystemValues, error) {
	s.Log("Reading crossystem values under test")
	var csParamsMap = map[reporters.CrossystemParam]*string{
		reporters.CrossystemParamDevBootUsb:         &cs.devBootUSBVal,
		reporters.CrossystemParamDevBootAltfw:       &cs.devBootAltfw,
		reporters.CrossystemParamFWUpdatetries:      &cs.fwUpdateTriesVal,
		reporters.CrossystemParamLocIdx:             &cs.locIdxVal,
		reporters.CrossystemParamBackupNvramRequest: &cs.backupNvramRequest,
	}
	for csKey, csVal := range csParamsMap {
		current, err := h.Reporter.CrossystemParam(ctx, csKey)
		if err != nil {
			return nil, err
		}
		*csVal = current
	}
	return &cs, nil
}

func setTargetCsVals(ctx context.Context, s *testing.State, h *firmware.Helper, targetMap map[string]string) error {
	targetArgs := make([]string, len(targetMap))
	i := 0
	for targetKey, targetVal := range targetMap {
		targetArgs[i] = fmt.Sprintf("%s=%s", targetKey, targetVal)
		i++
	}
	if err := h.DUT.Conn().CommandContext(ctx, "crossystem", targetArgs...).Run(); err != nil {
		return errors.Wrapf(err, "running crossystem %s", strings.Join(targetArgs, " "))
	}
	return nil
}

func equal(ctx context.Context, mapBefore, mapAfter map[string]string) (bool, error) {
	if len(mapBefore) != len(mapAfter) {
		return false, errors.New("Lengths of maps under evaluation do not match")
	}
	for k, v := range mapBefore {
		if elem, ok := mapAfter[k]; !ok || v != elem {
			return false, errors.Errorf("found mismatch in key %q, values are different: one has %q, while the other has %q", k, mapBefore[k], mapAfter[k])
		}
	}
	return true, nil
}

// logInformation logs some information for debugging purposes.
func logInformation(ctx context.Context, h *firmware.Helper, information string) (string, error) {
	var (
		err           error
		dutConnection servo.DUTConnTypeValue
		result        string
	)
	switch information {
	case "servoType":
		result, err = h.Servo.GetServoType(ctx)
	case "dutConnectionType":
		dutConnection, err = h.Servo.GetDUTConnectionType(ctx)
		result = string(dutConnection)
	case "pdCommunication":
		result, err = h.Servo.GetPDCommunication(ctx)
	default:
		result = "Unknown information"
	}
	if err != nil {
		return "", err
	}
	return result, nil
}

// openCCD attempts to open ccd if it's closed.
// To-do: we're adding a firmware helper function to check and open CCD via
// a more thorough process, i.e. verifying testlab status, ccd capabilities,
// and whether a servo micro is present.
func openCCD(ctx context.Context, h *firmware.Helper) error {
	if hasCCD, err := h.Servo.HasCCD(ctx); err != nil {
		return errors.Wrap(err, "while checking if servo has a CCD connection")
	} else if hasCCD {
		if val, err := h.Servo.GetString(ctx, servo.GSCCCDLevel); err != nil {
			return errors.Wrap(err, "failed to get gsc_ccd_level")
		} else if val != servo.Open {
			testing.ContextLogf(ctx, "CCD is not open, got %q. Attempting to unlock", val)
			if err := h.Servo.SetString(ctx, servo.CR50Testlab, servo.Open); err != nil {
				return errors.Wrap(err, "failed to unlock CCD")
			}
		}
	}
	return nil
}

// checkDUTAsleepAndPressPwr checks if DUT is at S5 or G3, and presses power button to boot it.
func checkDUTAsleepAndPressPwr(ctx context.Context, h *firmware.Helper) error {
	shortCtx, cancelShortCtx := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelShortCtx()
	testing.ContextLog(shortCtx, "Checking if power state is at S5 or G3")
	if err := h.WaitForPowerStates(shortCtx, firmware.PowerStateInterval, 1*time.Minute, "S5", "G3"); err != nil {
		return err
	}
	testing.ContextLogf(shortCtx, "Pressing power button for %s to wake DUT", h.Config.HoldPwrButtonPowerOn)
	if err := h.Servo.KeypressWithDuration(shortCtx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOn)); err != nil {
		return errors.Wrap(err, "failed to press power button")
	}
	return nil
}

// checkMaxChargerPower runs the local command 'power_supply_info' on DUT
// to check for max voltage and max current, and multiply them to get max power.
func checkMaxChargerPower(ctx context.Context, h *firmware.Helper) error {
	// Regular expressions.
	var (
		reMaxVoltage = regexp.MustCompile(`max\s+voltage\D*([^\n\r]*)`)
		reMaxCurrnet = regexp.MustCompile(`max\s+current\D*([^\n\r]*)`)
	)
	out, err := h.DUT.Conn().CommandContext(ctx, "power_supply_info").Output()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve power supply info from DUT")
	}
	findMatch := func(maxPowerVar string, pattern *regexp.Regexp, scannedOut []byte) (float64, error) {
		match := pattern.FindStringSubmatch(string(scannedOut))
		if len(match) < 2 {
			return 0, errors.Errorf("did not find value for %s", maxPowerVar)
		}
		val, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to parse for %s", maxPowerVar)
		}
		return val, nil
	}
	maxVoltage, err := findMatch("max_voltage", reMaxVoltage, out)
	if err != nil {
		return err
	}
	maxCurrent, err := findMatch("max_current", reMaxCurrnet, out)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "DUT receives max_voltage: %f, max_current: %f, max_power: %f",
		maxVoltage, maxCurrent, maxVoltage*maxCurrent)
	return nil
}

// checkChgstateBatt runs ec command 'chgstate' and collects information
// from the battery section.
func checkChgstateBatt(ctx context.Context, h *firmware.Helper, attr string) (string, error) {
	if err := h.Servo.RunECCommand(ctx, "chan 0"); err != nil {
		return "unknown", errors.Wrap(err, "failed to send 'chan 0' to EC")
	}
	defer func() {
		if err := h.Servo.RunECCommand(ctx, "chan 0xffffffff"); err != nil {
			testing.ContextLog(ctx, "Failed to send 'chan 0xffffffff' to EC: ", err)
		}
	}()
	match := `batt.*:((\n|.)*?is_present = \S*)[\n\r]`
	chgstateBatt, err := h.Servo.RunECCommandGetOutput(ctx, "chgstate", []string{match})
	if err != nil {
		return "unknwon", errors.Wrap(err, "failed to run command chgstate")
	}
	for _, val := range strings.Split(chgstateBatt[0][1], "\r\n\t") {
		if strings.Contains(val, attr) {
			return val, nil
		}
	}
	return "unknown", nil
}

// grepGpio runs ec command 'gpioget' to check for a gpio's value.
func grepGpio(ctx context.Context, h *firmware.Helper, name string) (string, error) {
	if err := h.Servo.RunECCommand(ctx, "chan 0"); err != nil {
		return "unknown", errors.Wrap(err, "failed to send 'chan 0' to EC")
	}
	defer func() {
		if err := h.Servo.RunECCommand(ctx, "chan 0xffffffff"); err != nil {
			testing.ContextLog(ctx, "Failed to send 'chan 0xffffffff' to EC: ", err)
		}
	}()
	match := fmt.Sprintf(`(?i)(0|1)[^\n\r]*\s%s`, name)
	cmd := fmt.Sprintf("gpioget %s", name)
	out, err := h.Servo.RunECCommandGetOutput(ctx, cmd, []string{match})
	if err != nil {
		return "unknown", errors.Wrapf(err, "failed to run command %v", cmd)
	}
	return out[0][1], nil
}

// checkPowerState checks for the dut's power state.
func checkPowerState(ctx context.Context, h *firmware.Helper) (string, error) {
	testing.ContextLog(ctx, "Checking for the DUT's power state")
	state, err := h.Servo.GetECSystemPowerState(ctx)
	if err != nil {
		return "unknown", err
	}
	return state, nil
}
