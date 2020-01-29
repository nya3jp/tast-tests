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

func setChargecontrol(ctx context.Context, s chargeControlState) error {
	if err := testexec.CommandContext(ctx, "ectool", "chargecontrol", string(s)).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "unable to set battery charge to %s", s)
	}
	return nil
}

// SetBatteryDischarge forces the battery to discharge. This will fail if the
// remaining battery is within lowBatteryMargin of the low power shutdown
// level.
func SetBatteryDischarge(ctx context.Context, lowBatteryMargin float64) (CleanupCallback, error) {
	testing.ContextLog(ctx, "Setting battery to discharge")
	low, err := power.LowBatteryShutdownPercent(ctx)
	if err != nil {
		return nil, err
	}
	b, err := power.NewBatteryState(ctx)
	if err != nil {
		return nil, err
	}
	if (low + lowBatteryMargin) >= b.ChargePercent() {
		return nil, errors.Errorf("battery percent %.2f is too low to start discharging", b.ChargePercent())
	}
	if b.Discharging() {
		testing.ContextLog(ctx, "WARNING Battery is already discharging")
	}
	if err := setChargecontrol(ctx, ccDischarge); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		// We reset the battery discharge mode to normal (charging), even if it
		// wasn't set before the test because leaving the device disharging
		// could cause a device to shut down.
		testing.ContextLog(ctx, "Resetting battery discharge to normal")
		return setChargecontrol(ctx, ccNormal)
	}, nil
}
