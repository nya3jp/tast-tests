// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

const batteryCheckInterval = time.Second

// BatteryInfoTracker is a helper to collect battery info.
type BatteryInfoTracker struct {
	prefix               string
	batteryPath          string
	chargeFullDesign     float64
	voltageMinDesign     float64
	batteryChargeStart   float64
	batteryChargeEnd     float64
	batteryCapacityStart float64
	batteryCapacityEnd   float64
	energy               float64 // Total energy consumed.
	energyFullDesign     float64
	collecting           chan bool
	collectingErr        chan error
	err                  error
}

// NewBatteryInfoTracker creates a new instance of BatteryInfoTracker. If battery is not
// used on the device, available flag is set to false and makes track a no-op.
func NewBatteryInfoTracker(ctx context.Context, metricPrefix string) (*BatteryInfoTracker, error) {
	batteryPath, err := power.SysfsBatteryPath(ctx)
	if err != nil {
		// Some devices (e.g. chromeboxes) do not have the battery, but that's fine
		// for now.
		// TODO(b/180915240): find the way to measure power data on those devices.
		testing.ContextLog(ctx, "Failed to get battery path: ", err)
		testing.ContextLog(ctx, "This might be okay. Continue the test without battery info")
		return nil, nil
	}

	chargeFullDesign, err := power.ReadBatteryProperty(batteryPath, "charge_full_design")
	if err != nil {
		return nil, err
	}
	voltageMinDesign, err := power.ReadBatteryProperty(batteryPath, "voltage_min_design")
	if err != nil {
		return nil, err
	}

	return &BatteryInfoTracker{
		prefix:           metricPrefix,
		batteryPath:      batteryPath,
		chargeFullDesign: chargeFullDesign,
		voltageMinDesign: voltageMinDesign,
	}, nil
}

// Start indicates that the battery tracking should start. It sets the batteryChargeStart value.
func (t *BatteryInfoTracker) Start(ctx context.Context) error {
	if t == nil {
		return nil
	}

	if t.collecting != nil {
		return errors.New("already started")
	}
	t.collecting = make(chan bool)
	t.collectingErr = make(chan error, 1)

	chargeNow, err := power.ReadBatteryProperty(t.batteryPath, "charge_now")
	if err != nil {
		return err
	}
	capacityNow, err := power.ReadBatteryCapacity(t.batteryPath)
	if err != nil {
		return err
	}

	t.batteryChargeStart = chargeNow
	t.batteryCapacityStart = capacityNow
	testing.ContextLogf(ctx, "charge_now value at start: %f, capacity value at start: %f", chargeNow, capacityNow)

	go func() {
		ticker := time.NewTicker(batteryCheckInterval)
		defer ticker.Stop()

		tOld := time.Now() // last collecting time.
		for {
			select {
			case <-t.collecting:
				close(t.collectingErr)
				return
			case <-ticker.C:
				watt, err := power.ReadSystemPower(t.batteryPath)
				if err != nil {
					t.collectingErr <- errors.Wrapf(err, "failed to read system power from %q", t.batteryPath)
					return
				}
				tNew := time.Now()
				t.energy += watt * tNew.Sub(tOld).Seconds()
				tOld = tNew
			case <-ctx.Done():
				t.collectingErr <- ctx.Err()
				return
			}
		}
	}()

	return nil
}

// Stop indicates that the battery tracking should stop. It sets the batteryChargeEnd value.
func (t *BatteryInfoTracker) Stop(ctx context.Context) error {
	if t == nil {
		return nil
	}

	if t.collecting == nil {
		return errors.New("not started")
	}

	chargeNow, err := power.ReadBatteryProperty(t.batteryPath, "charge_now")
	if err != nil {
		return err
	}
	capacityNow, err := power.ReadBatteryCapacity(t.batteryPath)
	if err != nil {
		return err
	}

	t.batteryChargeEnd = chargeNow
	t.batteryCapacityEnd = capacityNow
	testing.ContextLogf(ctx, "charge_now value at end: %f, capacity value at end: %f", chargeNow, capacityNow)

	t.energyFullDesign = t.chargeFullDesign * t.voltageMinDesign * 1e-12 * 3600

	// Stop energy collecting go routine.
	close(t.collecting)
	select {
	case err := <-t.collectingErr:
		if err != nil {
			// On boards like `drallion`, power.ReadSystemPower() could occasionally
			// fail. Record the error to skip reporting battery info for such boards.
			testing.ContextLog(ctx, "Energe collecting routine returned error: ", err)
			testing.ContextLog(ctx, "Battery info will not be reported")
			t.err = err
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// Record stores the collected data into pv for further processing.
func (t *BatteryInfoTracker) Record(pv *perf.Values) {
	if t == nil || t.err != nil {
		return
	}

	pv.Set(perf.Metric{
		Name:      t.prefix + "Battery.Charge.usage",
		Unit:      "microAh",
		Direction: perf.SmallerIsBetter,
	}, t.batteryChargeStart-t.batteryChargeEnd)
	pv.Set(perf.Metric{
		Name:      t.prefix + "Battery.Charge.fullDesign",
		Unit:      "microAh",
		Direction: perf.SmallerIsBetter,
	}, t.chargeFullDesign)
	pv.Set(perf.Metric{
		Name:      t.prefix + "Battery.Voltage.minDesign",
		Unit:      "microV",
		Direction: perf.SmallerIsBetter,
	}, t.voltageMinDesign)
	if t.chargeFullDesign != 0 {
		pv.Set(perf.Metric{
			Name:      t.prefix + "Battery.Charge.usagePercentage",
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, (t.batteryChargeStart-t.batteryChargeEnd)/t.chargeFullDesign*100)
	}
	pv.Set(perf.Metric{
		Name:      t.prefix + "Power.usage",
		Unit:      "J",
		Direction: perf.SmallerIsBetter,
	}, t.energy)
	if t.energyFullDesign != 0 {
		// Energy utilization percentage should be at the same level of
		// charge usage percentage. These two indicators can be cross checked.
		pv.Set(perf.Metric{
			Name:      t.prefix + "Power.usagePercentage",
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, t.energy/t.energyFullDesign*100)
	}
	pv.Set(perf.Metric{
		Name:      t.prefix + "Battery.Capacity.change",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, t.batteryCapacityStart-t.batteryCapacityEnd)
}
