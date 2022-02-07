// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FlagsPreservation,
		Desc:         "Checks that flag values are preserved over different power cycles",
		Contacts:     []string{"arthur.chuang@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.DevMode,
		SoftwareDeps: []string{"crossystem"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
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
	defer func() {
		s.Log("Restoring crossystem values to the original settings")
		if err := setTargetCsVals(ctx, s, h, originalCrossystemMap); err != nil {
			s.Fatal("Failed to restore crossystem values to the original settings")
		}
	}()

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
			s.Log("Power-cycling DUT by pressing power button")
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
				s.Fatal("Failed to set a keypress control by servo: ", err)
			}

			s.Log("Waiting for power state to become G3")
			if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3"); err != nil {
				s.Fatal("Failed to get powerstates at G3: ", err)
			}

			s.Log("Pressing on the power button to power on DUT")
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				s.Fatal("Failed to perform a tap on the power button: ", err)
			}
		case "powerCycleByRemovingBattery":
			s.Log("Power-cycling DUT by disconnecting AC and removing battery")
			if err := h.SetDUTPower(ctx, false); err != nil {
				s.Fatal("Failed to remove charger")
			}

			if err := h.Servo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
				s.Fatal("Failed to remove watchdog for ccd: ", err)
			}

			s.Log("Cutting off DUT's battery")
			cmd := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain)
			if err := cmd.BatteryCutoff(ctx); err != nil {
				s.Fatal("Failed to send the battery cutoff command: ", err)
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
		waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelWaitConnect()

		if h.WaitConnect(waitConnectCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}

		// Cr50 goes to sleep when the battery is disconnected, and when DUT wakes, CCD state might be locked.
		// Open CCD after supplying power and before talking to the EC.
		if val, err := h.Servo.GetString(ctx, servo.CR50CCDLevel); err != nil {
			s.Fatal("Failed to get cr50_ccd_level: ", err)
		} else if val != servo.Open {
			s.Logf("CCD is not open, got %q. Attempting to unlock", val)
			if err := h.Servo.SetString(ctx, servo.CR50Testlab, servo.Open); err != nil {
				s.Fatal("Failed to unlock CCD: ", err)
			}
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
