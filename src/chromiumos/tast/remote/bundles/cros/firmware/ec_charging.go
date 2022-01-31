// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"

	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECCharging,
		Desc:         "Servo based EC charging control test",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Fixture:      "bootModeNormal",
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.PowerMenuService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
	})
}

const (
	TrickleChargingThreshold = 100
)

// getChargingState returns map[string]string of parsed chgstate output
// from EC, in ideal situation this would be just predefined struct with
// fields for each value, but the EC console output seems to be varying
// between platforms and firmware versions in terms of fields' order,
// count and categories. This approach might look less elegant, but ii's
// safer and provides a nice interface to extract data

func getChargingState(ctx context.Context, s *testing.State, h *firmware.Helper) map[string]string {
	chgstateOutput, err := h.Servo.RunECCommandGetOutput(ctx, "chgstate", []string{`.*\ndebug output = .+\n`})
	if err != nil {
		s.Fatal("Failed querying EC: ", err)
	}

	var (
		category string
		key      string
		value    string
	)

	cstate_map := make(map[string]string)

	// For reference, the current output of "chgstate" EC command is provided below
	// in shortened form, actual field names and values might be different per board
	// If you notice any issues with parsing on newer EC firmware, change the parsing
	// method accordingly.
	// Example output of "chgstate":
	//   state = charge
	//   ac = 1
	//   batt_is_charging = 1
	//   chg.*:
	//     voltage = 8648mV
	//     current = 0mA
	//     (...)
	//   batt.*:
	//     temperature = 24C
	//     state_of_charge = 100%
	//     voltage = 8543mV
	//     current = 0mA
	//     (...)
	//   requested_voltage = 0mV
	//   requested_current = 0mA
	//   chg_ctl_mode = 0
	//   (...)
	for _, line := range strings.Split(chgstateOutput[0][0], "\n") {

		if strings.Contains(line, "*") {
			category = strings.Split(line, ".")[0]
		}
		if strings.Contains(line, "=") {
			if !strings.HasPrefix(line, "\t") {
				category = "global"
			}

			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSpace(line)
			key = strings.Split(line, " = ")[0]
			value = strings.Split(line, " = ")[1]

			cstate_map[category+"."+key] = value
		}
	}

	return cstate_map
}

func chargingInt(raw string, suffix string) (value int) {
	raw = strings.TrimSuffix(raw, suffix)
	value, _ = strconv.Atoi(raw)
	return value
}

// ECCharging discharges the DUT then checks its voltages
// and current to determine its charging circuitry and EC
// reporting is working as intended
func ECCharging(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.Servo.RunECCommand(ctx, "chan 0"); err != nil {
		s.Fatal("Failed to send 'chan 0' to EC: ", err)
	}

	defer func() {
		if err := h.Servo.RunECCommand(ctx, "chan 0xffffffff"); err != nil {
			s.Fatal("Failed to send 'chan 0xffffffff' to EC: ", err)
		}
	}()

	var cs = make(map[string]string)

	cs = getChargingState(ctx, s, h)
	if cs["global.ac"] != "1" {
		s.Fatal("DUT is not plugged to AC charger")
	}
	if cs["global.state"] != "charge" {
		s.Fatal("DUT is not charging (DUT is on AC but does not report charging)")
	}
	if chargingInt(cs["batt.current"], "mA") < 0 {
		s.Fatal("DUT is not charging (batterry current below zero)")
	}
	if (chargingInt(cs["batt.desired_current"], "mA") < TrickleChargingThreshold) &&
		(chargingInt(cs["batt.state_of_charge"], "%") < 100) {
		s.Fatalf("Trickling charging battery, unable to test (desired current: %s, threshold: %dmA)",
			cs["batt.desired_current"],
			TrickleChargingThreshold)
	}

	orig_pd_role, err := h.Servo.GetPDRole(ctx)
	if err != nil {
		s.Fatal("Failed to retrieve original USB PD role for Servo: ", err)
	}
	if orig_pd_role == servo.PDRoleNA {
		s.Fatal("Test requires Servo V4 or never to for operating DUT power delivery role through servo_pd_role")
	}

	s.Log("Initiating battery discharging")
	if err := h.Servo.SetPDRole(ctx, servo.PDRoleSnk); err != nil {
		s.Fatal("Failed to initialize battery discharging: ", err)
	}

	// As the firmware test with bootModeNormal does not receive
	// browser services on its initialization, we cannot easily
	// use Chrome for battery drain procedure. Instead, we can
	// simply spawn stress-ng (which seems to be available in
	// base rootfs) for specified amount of time
	// In the future, it might be more valuable to just create
	// the dedicated stressing service on DUT which will also
	// allow to monitor the battery status live

	const stressingScript = `
		cd /tmp; stress-ng --cpu 32 --timeout 4m
	`
	s.Log("Stressing CPU to discharge battery")
	if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", stressingScript).Run(); err != nil {
		s.Fatal("Failed to discharge battery using CPU stress: ", err)
	}

	cs = getChargingState(ctx, s, h)
	if float32(chargingInt(cs["chg.voltage"], "mV")) >= 1.05*float32(chargingInt(cs["batt.desired_voltage"], "mV")) {
		s.Fatalf("Charger target voltage is too high. (target: %s, battery: %s)",
			cs["chg.voltage"], cs["batt.desired_voltage"])
	}
	if float32(chargingInt(cs["chg.current"], "mA")) >= 1.05*float32(chargingInt(cs["batt.desired_current"], "mA")) {
		s.Fatalf("Charger target current is too high. (target: %s, battery: %s)",
			cs["chg.current"], cs["batt.desired_current"])
	}

	if float32(chargingInt(cs["batt.voltage"], "mV")) >= 1.05*float32(chargingInt(cs["chg.voltage"], "mV")) {
		s.Fatalf("Battery actual voltage is too high. (battery: %s, charger: %s",
			cs["batt.voltage"], cs["chg.voltage"])
	}
	if float32(chargingInt(cs["batt.current"], "mA")) >= 1.05*float32(chargingInt(cs["chg.current"], "mA")) {
		s.Fatalf("Battery actual current is too high. (battery: %s, charger: %s",
			cs["batt.current"], cs["chg.current"])
	}

	s.Log("Getting back to original USB PD role")
	if err := h.Servo.SetPDRole(ctx, orig_pd_role); err != nil {
		s.Fatal("Failed to get back to original USB PD role: ", err)
	}
}
