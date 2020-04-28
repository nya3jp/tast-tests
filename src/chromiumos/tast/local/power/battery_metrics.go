// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"io/ioutil"
	"path"
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
var ectoolBatteryRegexp = regexp.MustCompile(`^Battery info:
 +OEM name:               .*
 +Model number:           .*
 +Chemistry   :           .*
 +Serial number:          .*
 +Design capacity:        (\d+) mAh
 +Last full charge:       \d+ mAh
 +Design output voltage   \d+ mV
 +Cycle count             \d+
 +Present voltage         (\d+) mV
 +Present current         (\d+) mA
 +Remaining capacity      (\d+) mAh
 +Flags                   0x([0-9A-Fa-f]+).*
$`)

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

// GetSystemPowerFromSysfs returns system power consumption in Watt.
// If there are no batteries support reporting voltage_now and current_now,
// the return value is 0., otherwise sum of power consumption of each battery.
func GetSystemPowerFromSysfs() (float64, error) {
	systemPower := 0.
	const sysFsPowerSupplyPath = "/sys/class/power_supply"
	files, err := ioutil.ReadDir(sysFsPowerSupplyPath)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read %v: %v", sysFsPowerSupplyPath, err)
	}
	readLine := func(devPath, name string) (string, error) {
		strBytes, err := ioutil.ReadFile(path.Join(devPath, name))
		if err != nil {
			return "", err
		}
		return strings.TrimSuffix(string(strBytes), "\n"), nil
	}
	readFloat64 := func(devPath, name string) (float64, error) {
		str, err := readLine(devPath, name)
		if err != nil {
			return 0., err
		}
		return strconv.ParseFloat(str, 64)
	}
	for _, file := range files {
		devPath := path.Join(sysFsPowerSupplyPath, file.Name())
		supplyType, err := readLine(devPath, "type")
		if err != nil || !strings.HasPrefix(supplyType, "Battery") {
			continue
		}
		supplyVoltage, err := readFloat64(devPath, "voltage_now")
		if err != nil {
			continue
		}
		supplyCurrent, err := readFloat64(devPath, "current_now")
		if err != nil {
			continue
		}
		// voltage_now and current_now reports their value in micro unit
		// so adjust this to match with Watt.
		systemPower += supplyVoltage * supplyCurrent * 1e-12
	}
	return systemPower, nil
}

// BatteryMetrics hold the results of once call to ectool and enough
// information to compute battery drain over time.
type BatteryMetrics struct {
	systemPowerMetric        perf.Metric
	reportsSystemPowerMetric bool
	voltageMetric            perf.Metric
	currentMetric            perf.Metric
	powerMetric              perf.Metric
	energyMetric             perf.Metric
	lowSys                   float64
	lowMargin                float64
	prevState                *BatteryState
}

var _ perf.TimelineDatasource = &BatteryMetrics{}

// NewBatteryMetrics creates a struct to capture battery metrics with the
// ectool command.
func NewBatteryMetrics(lowBatteryMargin float64) *BatteryMetrics {
	return &BatteryMetrics{
		lowSys:    100.0,
		lowMargin: lowBatteryMargin,
		prevState: nil,
	}
}

// Setup reads the low battery shutdown percent that that we can error out a
// test if the battery is ever too low.
func (b *BatteryMetrics) Setup(ctx context.Context, prefix string) error {
	low, err := LowBatteryShutdownPercent(ctx)
	if err != nil {
		return err
	}
	b.lowSys = low
	b.systemPowerMetric = perf.Metric{Name: prefix + "system_power", Unit: "W", Direction: perf.SmallerIsBetter, Multiple: true}
	b.voltageMetric = perf.Metric{Name: prefix + "ectool_battery_voltage", Unit: "mV", Direction: perf.SmallerIsBetter, Multiple: true}
	b.currentMetric = perf.Metric{Name: prefix + "ectool_battery_current", Unit: "mA", Direction: perf.SmallerIsBetter, Multiple: true}
	b.powerMetric = perf.Metric{Name: prefix + "ectool_battery_power", Unit: "mW", Direction: perf.SmallerIsBetter, Multiple: true}
	b.energyMetric = perf.Metric{Name: prefix + "ectool_battery_energy", Unit: "mAh", Direction: perf.SmallerIsBetter, Multiple: true}
	return nil
}

// Start captures the initial battery state which the first snapshot will be
// relative to.
func (b *BatteryMetrics) Start(ctx context.Context) error {
	// Check if the system can report system_power through sysfs
	_, err := GetSystemPowerFromSysfs()
	b.reportsSystemPowerMetric = err == nil

	state, err := NewBatteryState(ctx)
	if err != nil {
		return err
	}
	b.prevState = state
	return nil
}

// Snapshot takes a snapshot of battery metrics. It also checks that the
// battery is not too low for the test to continue.
func (b *BatteryMetrics) Snapshot(ctx context.Context, values *perf.Values) error {
	if b.reportsSystemPowerMetric {
		// Report system_power only if sysfs interface exists.
		power, err := GetSystemPowerFromSysfs()
		if err != nil {
			return err
		}
		values.Append(b.systemPowerMetric, power)
	}
	state, err := NewBatteryState(ctx)
	if err != nil {
		return err
	}
	if state.ChargePercent() <= (b.lowSys + b.lowMargin) {
		return errors.Errorf("battery percent %.2f is too low", state.ChargePercent())
	}
	values.Append(b.voltageMetric, state.Voltage)
	values.Append(b.currentMetric, state.Current)
	values.Append(b.powerMetric, state.Power())
	values.Append(b.energyMetric, b.prevState.Remaining-state.Remaining)
	b.prevState = state
	return nil
}
