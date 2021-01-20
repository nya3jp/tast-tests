// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func hasSysfsAttribute(filePath string) bool {
	_, err := os.Stat(filePath)
	return err != nil
}

// LowBatteryShutdownPercent gets the battery percentage below which the system
// turns off.
func LowBatteryShutdownPercent(ctx context.Context) (float64, error) {
	output, err := testexec.CommandContext(ctx,
		"check_powerd_config",
		"--low_battery_shutdown_percent").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to get low battery shutdown percent")
	}
	percent, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0.0, errors.Wrapf(err, "failed to parse low battery shutdown percent from %q", output)
	}
	return percent, nil
}

// BatteryStatus represents a charging status of a battery.
type BatteryStatus int

// These values are corresponds to status attribute of sysfs power_supply.
const (
	BatteryStatusUnknown BatteryStatus = iota
	BatteryStatusCharging
	BatteryStatusDischarging
	BatteryStatusNotCharging
	BatteryStatusFull
)

var batteryStatusMap = map[string]BatteryStatus{
	"Unknown":      BatteryStatusUnknown,
	"Charging":     BatteryStatusCharging,
	"Discharging":  BatteryStatusDischarging,
	"Not charging": BatteryStatusNotCharging,
	"Full":         BatteryStatusFull,
}

// ReadBatteryStatus returns the current battery status.
func ReadBatteryStatus(devPath string) (BatteryStatus, error) {
	statusStr, err := readFirstLine(path.Join(devPath, "status"))
	if err != nil {
		return BatteryStatusUnknown, errors.Errorf("%v lacks status attribute", devPath)
	}
	status, ok := batteryStatusMap[statusStr]
	if !ok {
		return BatteryStatusUnknown, errors.Errorf("status %v is not expected", statusStr)
	}
	return status, nil
}

// ReadBatteryCapacity returns the percentage of current charge of a battery
// which comes from /sys/class/power_supply/<supply name>/capacity.
func ReadBatteryCapacity(devPath string) (float64, error) {
	capacity, err := readInt64(path.Join(devPath, "capacity"))
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read capacity from %v", devPath)
	}
	return float64(capacity), nil
}

// ReadBatteryChargeNow returns the charge of a battery in Ah.
// which comes from /sys/class/power_supply/<supply name>/charge_now.
func ReadBatteryChargeNow(devPath string) (float64, error) {
	charge, err := readInt64(path.Join(devPath, "charge_now"))
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read charge from %v", devPath)
	}
	return float64(charge) / 1000000, nil
}

// ReadBatteryEnergy returns the remaining energy of a battery in Wh.
func ReadBatteryEnergy(devPath string) (float64, error) {
	charge, err := ReadBatteryChargeNow(devPath)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read energy from %v", devPath)
	}

	voltage, err := readFloat64(path.Join(devPath, "voltage_min_design"))
	if err != nil {
		// autotest/files/client/cros/power/power_status.py uses current
		// voltage in case it can't get the specced one.
		return 0., errors.Wrap(err, "failed to read voltage_min_design")
	}
	return charge * float64(voltage) / 1000000, nil
}

// ReadSystemPower returns system power consumption in Watts.
// It is assumed that power supplies at devPath have attributes
// voltage_now and current_now.
// If reading these attributes fails, this function returns non-nil error,
// otherwise returns power consumption of the battery.
func ReadSystemPower(devPath string) (float64, error) {
	supplyVoltage, err := readFloat64(path.Join(devPath, "voltage_now"))
	if err != nil {
		return 0., errors.Wrap(err, "failed to read voltage_now")
	}
	supplyCurrent, err := readFloat64(path.Join(devPath, "current_now"))
	if err != nil {
		return 0., errors.Wrap(err, "failed to read current_now")
	}
	if supplyCurrent < 0. {
		// Some board (e.g. hana) reports negative values for current_now
		// when discharging so flip the sign on that case to align with
		// other boards.
		supplyCurrent = -supplyCurrent
	}
	// voltage_now and current_now reports their value in micro unit
	// so adjust this to match with Watt.
	return supplyVoltage * supplyCurrent * 1e-12, nil
}

// listSysfsBatteryPaths lists paths of batteries which supply power to the system
// and has voltage_now and current_now attributes.
func listSysfsBatteryPaths(ctx context.Context) ([]string, error) {
	// TODO(hikarun): Remove ContextLogf()s after checking this function works on all platforms
	const sysfsPowerSupplyPath = "/sys/class/power_supply"
	testing.ContextLog(ctx, "Listing batteries in ", sysfsPowerSupplyPath)
	files, err := ioutil.ReadDir(sysfsPowerSupplyPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read sysfs dir")
	}
	var batteryPaths []string
	for _, file := range files {
		devPath := path.Join(sysfsPowerSupplyPath, file.Name())
		supplyType, err := readFirstLine(path.Join(devPath, "type"))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read type of %v", devPath)
		}
		if supplyType != "Battery" {
			testing.ContextLogf(ctx, "%v is not a Battery", devPath)
			continue
		}
		supplyScope, err := readFirstLine(path.Join(devPath, "scope"))
		if err != nil && !os.IsNotExist(err) {
			// Ignore NotExist error since /sys/class/power_supply/*/scope may not exist
			return nil, errors.Wrapf(err, "failed to read scope of %v", devPath)
		}
		if supplyScope == "Device" {
			// Ignore batteries for peripheral devices.
			testing.ContextLogf(ctx, "%v is a Battery with Device scope", devPath)
			continue
		}
		if !hasSysfsAttribute(path.Join(sysfsPowerSupplyPath, "voltage_now")) ||
			!hasSysfsAttribute(path.Join(sysfsPowerSupplyPath, "current_now")) {
			testing.ContextLogf(ctx, "%v lacks voltage_now or current_now", devPath)
			continue
		}
		batteryPaths = append(batteryPaths, devPath)
	}
	return batteryPaths, nil
}

// SysfsBatteryPath returns a path of battery which supply power to the system
// and has voltage_now and current_now attributes.
func SysfsBatteryPath(ctx context.Context) (string, error) {
	batteryPaths, err := listSysfsBatteryPaths(ctx)
	if err != nil {
		return "", err
	}
	if len(batteryPaths) != 1 {
		return "", errors.Errorf("unexpected number of batteries: got %d; want 1", len(batteryPaths))
	}
	return batteryPaths[0], nil
}

// SysfsBatteryMetrics hold the metrics read from sysfs.
type SysfsBatteryMetrics struct {
	powerMetric perf.Metric
	batteryPath string
}

// Assert that SysfsBatteryMetrics can be used in perf.Timeline.
var _ perf.TimelineDatasource = &SysfsBatteryMetrics{}

// NewSysfsBatteryMetrics creates a struct to capture battery metrics with sysfs.
func NewSysfsBatteryMetrics() *SysfsBatteryMetrics {
	return &SysfsBatteryMetrics{}
}

// Setup reads the low battery shutdown percent that that we can error out a
// test if the battery is ever too low.
func (b *SysfsBatteryMetrics) Setup(ctx context.Context, prefix string) error {
	batteryPaths, err := listSysfsBatteryPaths(ctx)
	if err != nil {
		return err
	}
	if len(batteryPaths) == 0 {
		testing.ContextLog(ctx, "Not reporting 'system' metric since no batteries found")
		return nil
	}
	testing.ContextLogf(ctx, "SysfsBatteryMetrics uses %v batteries:", len(batteryPaths))
	for _, path := range batteryPaths {
		testing.ContextLog(ctx, path)
	}
	if len(batteryPaths) != 1 {
		return errors.Errorf("unexpected number of batteries: got %d; want 1", len(batteryPaths))
	}
	b.batteryPath = batteryPaths[0]
	b.powerMetric = perf.Metric{Name: prefix + "system", Unit: "W", Direction: perf.SmallerIsBetter, Multiple: true}
	return nil
}

// Start captures the initial battery state which the first snapshot will be
// relative to.
func (b *SysfsBatteryMetrics) Start(ctx context.Context) error {
	return nil
}

// Snapshot takes a snapshot of battery metrics.
// If there are no batteries can be used to report the metrics,
// Snapshot does nothing and returns without error.
func (b *SysfsBatteryMetrics) Snapshot(ctx context.Context, values *perf.Values) error {
	power, err := ReadSystemPower(b.batteryPath)
	if err != nil {
		return err
	}
	values.Append(b.powerMetric, power)
	return nil
}
