// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/power/ectool"
	"chromiumos/tast/local/testexec"
)

// setBatteryDischarge is an Action that forces the battery to
// discharge.
type setBatteryDischarge struct {
	ctx              context.Context
	lowBatteryMargin float64
}

// Setup checks that the battery has enough charge, and forces discharge.
func (a *setBatteryDischarge) Setup() error {
	low, err := ectool.LowBatteryShutdownPercent(a.ctx)
	if err != nil {
		return err
	}
	b, err := ectool.NewBatteryState(a.ctx)
	if err != nil {
		return err
	}
	if (low + a.lowBatteryMargin) >= b.ChargePercent() {
		return errors.Errorf("battery percent %.2f is too low to start discharging", b.ChargePercent())
	}
	if b.Discharging() {
		a.Cleanup()
		return errors.New("battery is already discharging")
	}
	if err := testexec.CommandContext(a.ctx, "ectool", "chargecontrol", "discharge").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to force battery discharge")
	}
	return nil
}

// Cleanup reenables charging.
func (a *setBatteryDischarge) Cleanup() error {
	if err := testexec.CommandContext(a.ctx, "ectool", "chargecontrol", "normal").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to disable battery discharge")
	}
	return nil
}

// SetBatteryDischarge creates an Action to force battery discharge.
func SetBatteryDischarge(ctx context.Context, lowBatteryMargin float64) Action {
	return &setBatteryDischarge{
		ctx:              ctx,
		lowBatteryMargin: lowBatteryMargin,
	}
}
