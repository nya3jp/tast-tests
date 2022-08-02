// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type chargeControlState struct {
	state          string
	expectedOutput string
}

var (
	ccDischarge = chargeControlState{"discharge", "Charge state machine force discharge."}
	ccNormal    = chargeControlState{"normal", "Charge state machine is in normal mode."}
)

func setChargeControl(ctx context.Context, s chargeControlState) error {
	stdout, stderr, err := testexec.CommandContext(ctx, "ectool", "chargecontrol", s.state).SeparatedOutput(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "unable to set battery charge to %s, got error %s", s.state, string(stderr))
	}
	if output := strings.TrimSpace(string(stdout)); output != s.expectedOutput {
		return errors.Errorf("unexpected output: got %s; want %s", output, s.expectedOutput)
	}
	return nil
}

// SetBatteryDischarge forces the battery to discharge. This will fail if the
// remaining battery charge is lower than lowBatteryCutoff.
func SetBatteryDischarge(ctx context.Context, expectedMaxCapacityDischarge float64) (CleanupCallback, error) {
	shutdownCutoff, err := power.LowBatteryShutdownPercent(ctx)
	if err != nil {
		return nil, err
	}
	lowBatteryCutoff := shutdownCutoff + expectedMaxCapacityDischarge
	devPath, err := power.SysfsBatteryPath(ctx)
	if err != nil {
		return nil, err
	}
	capacity, err := power.ReadBatteryCapacity(devPath)
	if err != nil {
		return nil, err
	}
	energy, err := power.ReadBatteryEnergy(devPath)
	if err != nil {
		return nil, err
	}
	status, err := power.ReadBatteryStatus(devPath)
	if err != nil {
		return nil, err
	}
	if status == power.BatteryStatusDischarging {
		testing.ContextLog(ctx, "WARNING Battery is already discharging")
	}

	testing.ContextLog(ctx, "Setting battery to discharge. Current capacity: ", capacity, "% (", energy, "Wh), Shutdown cutoff: ", shutdownCutoff, "%, Expected maximum discharge during test: ", expectedMaxCapacityDischarge, "%")
	if lowBatteryCutoff >= capacity {
		return nil, errors.Errorf("battery percent %.2f is too low to start discharging", capacity)
	}

	if err := setChargeControl(ctx, ccDischarge); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		dischargePercent := 0.0
		dischargeWh := 0.0
		capacityAfterTest, err := power.ReadBatteryCapacity(devPath)
		if err == nil {
			dischargePercent = capacity - capacityAfterTest
		}
		energyAfterTest, err := power.ReadBatteryEnergy(devPath)
		if err == nil {
			dischargeWh = energy - energyAfterTest
		}
		// We reset the battery discharge mode to normal (charging), even if it
		// wasn't set before the test because leaving the device disharging
		// could cause a device to shut down.
		testing.ContextLog(ctx, "Resetting battery discharge to normal. Discharge during test: ", dischargePercent, "% (", dischargeWh, "Wh)")
		return setChargeControl(ctx, ccNormal)
	}, nil
}
