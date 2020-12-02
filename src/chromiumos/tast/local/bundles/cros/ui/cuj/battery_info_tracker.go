// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

// BatteryInfoTracker is a helper to collect battery info.
type BatteryInfoTracker struct {
	prefix             string
	batteryPaths       []string
	chargeFullDesign   float64
	voltageMinDesign   float64
	batteryChargeStart float64
	batteryChargeEnd   float64
	energy             float64 // Total energy consumed.
	energyFullDesign   float64
	timeline           *perf.Timeline // Timeline to periodically read system battery metrics.
}

// NewBatteryInfoTracker creates a new instance of BatteryInfoTracker. If battery is not
// used on the device, available flag is set to false and makes track a no-op.
func NewBatteryInfoTracker(ctx context.Context, metricPrefix string) (*BatteryInfoTracker, error) {
	batteryPaths, err := power.ListSysfsBatteryPaths(ctx)
	if err != nil {
		return nil, err
	}
	if len(batteryPaths) == 0 {
		// no battery available.
		return nil, nil
	}

	chargeFullDesign, err := power.ReadBatteryPropertyInt64(batteryPaths, "charge_full_design")
	if err != nil {
		return nil, err
	}
	voltageMinDesign, err := power.ReadBatteryPropertyInt64(batteryPaths, "voltage_min_design")
	if err != nil {
		return nil, err
	}

	sources := []perf.TimelineDatasource{
		power.NewSysfsBatteryMetricsWithPrefix("Battery."),
	}
	timeline, err := perf.NewTimeline(ctx, sources, perf.Interval(checkInterval), perf.Prefix(metricPrefix))
	if err != nil {
		return nil, errors.Wrap(err, "failed to start perf.Timeline")
	}

	return &BatteryInfoTracker{
		prefix:           metricPrefix,
		batteryPaths:     batteryPaths,
		chargeFullDesign: chargeFullDesign,
		voltageMinDesign: voltageMinDesign,
		timeline:         timeline,
	}, nil
}

// Start indicates that the battery tracking should start. It sets the batteryChargeStart value.
func (t *BatteryInfoTracker) Start(ctx context.Context) error {
	if t == nil {
		return nil
	}

	chargeNow, err := power.ReadBatteryPropertyInt64(t.batteryPaths, "charge_now")
	if err != nil {
		return err
	}

	t.batteryChargeStart = chargeNow
	testing.ContextLog(ctx, "charge_now value at start: ", chargeNow)

	if err := t.timeline.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start perf.Timeline")
	}
	if err := t.timeline.StartRecording(ctx); err != nil {
		return errors.Wrap(err, "failed to start recording timeline data")
	}
	return nil
}

// Stop indicates that the battery tracking should stop. It sets the batteryChargeEnd value.
func (t *BatteryInfoTracker) Stop(ctx context.Context) error {
	if t == nil {
		return nil
	}

	chargeNow, err := power.ReadBatteryPropertyInt64(t.batteryPaths, "charge_now")
	if err != nil {
		return err
	}

	t.batteryChargeEnd = chargeNow
	testing.ContextLog(ctx, "charge_now value at end: ", chargeNow)

	vs, err := t.timeline.StopRecording()
	if err != nil {
		return errors.Wrap(err, "failed to stop recording timeline data")
	}
	joules := float64(0)
	powerSystemMetricName := metricPrefix + "Battery." + "system"
	if watts := vs.Get(powerSystemMetricName); watts != nil {
		for _, v := range watts {
			joules += float64(v) * checkInterval.Seconds()
		}
		t.energy = joules
	}

	t.energyFullDesign = t.chargeFullDesign * t.voltageMinDesign * 1e-12 * 3600

	return nil
}

// Record stores the collected data into pv for further processing.
func (t *BatteryInfoTracker) Record(pv *perf.Values) {
	if t == nil {
		return
	}

	pv.Set(perf.Metric{
		Name:      t.prefix + "Battery.Charge.usage",
		Unit:      "microAh",
		Direction: perf.SmallerIsBetter,
	}, float64(t.batteryChargeStart-t.batteryChargeEnd))
	pv.Set(perf.Metric{
		Name:      t.prefix + "Battery.Charge.fullDesign",
		Unit:      "microAh",
		Direction: perf.SmallerIsBetter,
	}, float64(t.chargeFullDesign))
	pv.Set(perf.Metric{
		Name:      t.prefix + "Battery.Voltage.minDesign",
		Unit:      "microV",
		Direction: perf.SmallerIsBetter,
	}, float64(t.voltageMinDesign))
	if t.chargeFullDesign != 0 {
		pv.Set(perf.Metric{
			Name:      t.prefix + "Battery.Charge.usagePercentage",
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, float64((t.batteryChargeStart-t.batteryChargeEnd)/t.chargeFullDesign*100))
	}
	pv.Set(perf.Metric{
		Name:      metricPrefix + "Power.usage",
		Unit:      "J",
		Direction: perf.SmallerIsBetter,
	}, t.energy)
	if t.energyFullDesign != 0 {
		pv.Set(perf.Metric{
			Name:      metricPrefix + "Power.usagePercentage",
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, float64(t.energy/t.energyFullDesign*100))
	}
}
