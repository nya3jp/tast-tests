// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package power interacts with power management on behalf of local tests.
package power

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// Status holds power supply information reported by powerd's dump_power_status
// tool.
type Status struct {
	LinePowerConnected bool
	LinePowerCurrent   float64
	LinePowerType      string

	BatteryPresent          bool
	BatteryDischarging      bool
	BatteryPercent          float64
	BatteryDisplayPercent   float64
	BatteryCharge           float64
	BatteryChargeFull       float64
	BatteryChargeFullDesign float64
	BatteryCurrent          float64
	BatteryEnergy           float64
	BatteryEnergyRate       float64
	BatteryVoltage          float64
	BatteryStatus           string
}

// GetStatus returns current power supply information.
func GetStatus(ctx context.Context) (*Status, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := testexec.CommandContext(ctx, "dump_power_status")
	b, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return nil, err
	}

	m := make(map[string]string)

	getNumValue := func(k string) float64 {
		if err != nil {
			return 0.0
		}
		s := m[k]
		v := 0.0
		v, err = strconv.ParseFloat(s, 64)
		if err != nil {
			err = errors.Errorf("key %q has non-float value %q", k, s)
			return 0.0
		}
		return v
	}

	for _, l := range strings.Split(string(b), "\n") {
		if l == "" {
			break
		}
		// The name and value are separated by a single space, but the value
		// may be a string containing additional spaces.
		f := strings.SplitN(l, " ", 2)
		if len(f) != 2 {
			return nil, errors.Errorf("didn't find two fields in line %q", l)
		}
		m[f[0]] = f[1]
	}

	s := &Status{
		LinePowerConnected:      getNumValue("line_power_connected") != 0.0,
		LinePowerCurrent:        getNumValue("line_power_current"),
		LinePowerType:           m["line_power_type"],
		BatteryPresent:          getNumValue("battery_present") != 0.0,
		BatteryDischarging:      getNumValue("battery_discharging") != 0.0,
		BatteryPercent:          getNumValue("battery_percent"),
		BatteryDisplayPercent:   getNumValue("battery_display_percent"),
		BatteryCharge:           getNumValue("battery_charge"),
		BatteryChargeFull:       getNumValue("battery_charge_full"),
		BatteryChargeFullDesign: getNumValue("battery_charge_full_design"),
		BatteryCurrent:          getNumValue("battery_current"),
		BatteryEnergy:           getNumValue("battery_energy"),
		BatteryEnergyRate:       getNumValue("battery_energy_rate"),
		BatteryVoltage:          getNumValue("battery_voltage"),
		BatteryStatus:           m["battery_status"],
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}
