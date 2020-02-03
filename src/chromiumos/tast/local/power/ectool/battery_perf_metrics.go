// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ectool contains helper functions to interface with ectool.
package ectool

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
)

// BatteryState holds all interesting battery metrics.
type BatteryState struct {
	Capacity  float64
	Remaining float64
	Voltage   float64
	Current   float64
	Flags     uint8
}

// Power computes the power in mW of a BatteryState.
func (b *BatteryState) Power() float64 {
	// milliInOne is used to renormalize the result of the multiplication back
	// to millis. Without renormalizing, result would be in millimillis/micros.
	const milliInOne = 0.001
	return b.Voltage * b.Current * milliInOne
}

// ChargePercent computes how full the battery is.
func (b *BatteryState) ChargePercent() float64 {
	return b.Remaining / b.Capacity * 100.0
}

// Discharging checks if a BatteryState is discharging.
func (b *BatteryState) Discharging() bool {
	const dischargingBit = 0x04
	return (b.Flags & dischargingBit) != 0
}

// LowBatteryShutdownPercent gets the battery percentage below which the system
// turns off.
func LowBatteryShutdownPercent(ctx context.Context) (float64, error) {
	output, err := testexec.CommandContext(ctx, "check_powerd_config", "--low_battery_shutdown_percent").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to get low battery shutdown percent")
	}
	percent, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0.0, errors.Wrapf(err, "failed to parse low battery shutdown percent from %q", output)
	}
	return percent, nil
}

// ectoolBatteryRegexp is used to parse the results of an 'ectool battery'
// command execution.
var ectoolBatteryRegexp = regexp.MustCompile("^Battery info:\n" +
	" +OEM name:               .*\n" +
	" +Model number:           .*\n" +
	" +Chemistry   :           .*\n" +
	" +Serial number:          .*\n" +
	" +Design capacity:        (\\d+) mAh\n" +
	" +Last full charge:       \\d+ mAh\n" +
	" +Design output voltage   \\d+ mV\n" +
	" +Cycle count             \\d+\n" +
	" +Present voltage         (\\d+) mV\n" +
	" +Present current         (\\d+) mA\n" +
	" +Remaining capacity      (\\d+) mAh\n" +
	" +Flags                   0x([0-9A-Fa-f]+).*\n$")

const (
	capacityReIndex  = 1
	voltageReIndex   = 2
	currentReIndex   = 3
	remainingReIndex = 4
	flagsReIndex     = 5
)

// NewBatteryState executes an 'ectool battery' command and parses the results
// to make a new BatteryState snapshot.
func NewBatteryState(ctx context.Context) (*BatteryState, error) {
	output, err := testexec.CommandContext(ctx, "ectool", "battery").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call ectool battery")
	}
	match := ectoolBatteryRegexp.FindSubmatch(output)
	if match == nil {
		return nil, errors.Wrapf(err, "failed to parse ectool battery results %q", output)
	}
	capacity, err := strconv.ParseFloat(string(match[capacityReIndex]), 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse capacity %q", match[capacityReIndex])
	}
	voltage, err := strconv.ParseFloat(string(match[voltageReIndex]), 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse voltage %q", match[voltageReIndex])
	}
	current, err := strconv.ParseFloat(string(match[currentReIndex]), 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse current %q", match[currentReIndex])
	}
	remaining, err := strconv.ParseFloat(string(match[remainingReIndex]), 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse remaining %q", match[remainingReIndex])
	}
	flags, err := strconv.ParseUint(string(match[flagsReIndex]), 16, 8)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse flags %q", match[flagsReIndex])
	}
	return &BatteryState{
		Capacity:  capacity,
		Remaining: remaining,
		Voltage:   voltage,
		Current:   current,
		Flags:     uint8(flags),
	}, nil
}

// BatteryMetrics hold the results of once call to ectool and enough
// information to compute battery drain over time.
type BatteryMetrics struct {
	ctx           context.Context
	voltageMetric perf.Metric
	currentMetric perf.Metric
	powerMetric   perf.Metric
	lowSys        float64
	lowMargin     float64
}

// NewBatteryMetrics creates a struct to capture battery metrics with the
// ectool command.
func NewBatteryMetrics(ctx context.Context, lowBatteryMargin float64) *BatteryMetrics {
	return &BatteryMetrics{
		ctx:           ctx,
		voltageMetric: perf.Metric{Name: "ectool_battery_voltage", Unit: "mV", Direction: perf.SmallerIsBetter, Multiple: true},
		currentMetric: perf.Metric{Name: "ectool_battery_current", Unit: "mA", Direction: perf.SmallerIsBetter, Multiple: true},
		powerMetric:   perf.Metric{Name: "ectool_battery_power", Unit: "mW", Direction: perf.SmallerIsBetter, Multiple: true},
		lowSys:        100.0,
		lowMargin:     lowBatteryMargin,
	}
}

// Setup reads the low battery shutdown percent that that we can error out a
// test if the battery is ever too low.
func (b *BatteryMetrics) Setup() error {
	low, err := LowBatteryShutdownPercent(b.ctx)
	if err != nil {
		return err
	}
	b.lowSys = low
	return nil
}

// Snapshot takes a snapshot of battery metrics. It also checks that the
// battery is not too low for the test to continue.
func (b *BatteryMetrics) Snapshot(values *perf.Values) error {
	state, err := NewBatteryState(b.ctx)
	if err != nil {
		return err
	}
	if state.ChargePercent() <= (b.lowSys + b.lowMargin) {
		return errors.Errorf("battery percent %.2f is too low", state.ChargePercent())
	}
	values.Append(b.voltageMetric, state.Voltage)
	values.Append(b.currentMetric, state.Current)
	values.Append(b.powerMetric, state.Power())
	return nil
}
