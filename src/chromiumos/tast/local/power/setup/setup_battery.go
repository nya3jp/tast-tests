// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type chargeControlState string

const (
	ccDischarge chargeControlState = "discharge"
	ccNormal    chargeControlState = "normal"
)

func setChargeControl(ctx context.Context, s chargeControlState) error {
	if err := testexec.CommandContext(ctx, "ectool", "chargecontrol", string(s)).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "unable to set battery charge to %s", s)
	}
	return nil
}

// SetBatteryDischarge forces the battery to discharge. This will fail if the
// remaining battery charge is lower than lowBatteryCutoff.
func SetBatteryDischarge(ctx context.Context, lowBatteryMargin float64) (CleanupCallback, error) {
	shutdownCutoff, err := power.LowBatteryShutdownPercent(ctx)
	if err != nil {
		return nil, err
	}
	lowBatteryCutoff := shutdownCutoff + lowBatteryMargin
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

	testing.ContextLog(ctx, "Setting battery to discharge. Current capacity: ", capacity, "% (", energy, "Wh), Shutdown cutoff: ", shutdownCutoff, "%, Expected maximum discharge during test: ", lowBatteryMargin, "%")
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
		err = setChargeControl(ctx, ccNormal)

		if dischargePercent > lowBatteryMargin {
			return errors.Wrapf(err, "discharge is greater than expected: %f", lowBatteryMargin)
		}
		return err
	}, nil
}
