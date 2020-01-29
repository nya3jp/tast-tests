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

func setChargecontrolDischarge(ctx context.Context) error {
	if err := testexec.CommandContext(ctx, "ectool", "chargecontrol", "discharge").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to force battery discharge")
	}
	return nil
}

func setChargecontrolNormal(ctx context.Context) error {
	if err := testexec.CommandContext(ctx, "ectool", "chargecontrol", "normal").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to disable battery discharge")
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
	if err := setChargecontrolDischarge(ctx); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		testing.ContextLog(ctx, "Resetting battery discharge to normal")
		return setChargecontrolNormal(ctx)
	}, nil
}
