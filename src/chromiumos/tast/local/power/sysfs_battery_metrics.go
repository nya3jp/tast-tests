// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"io/ioutil"
	"os"
	"path"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func hasSysfsAttribute(filePath string) bool {
	_, err := os.Stat(filePath)
	return err != nil
}

// readSystemPower returns system power consumption in Watt.
// It is assumed that power supplies listed in devPaths have attributes
// voltage_now and current_now. If reading these attributes fails, this function
// returns non-nil error, otherwise returns sum of power consumption of each battery.
func readSystemPower(devPaths []string) (float64, error) {
	systemPower := 0.
	for _, devPath := range devPaths {
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
		systemPower += supplyVoltage * supplyCurrent * 1e-12
	}
	return systemPower, nil
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
		supplyType, err := readLine(path.Join(devPath, "type"))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read type of %v", devPath)
		}
		if supplyType != "Battery" {
			testing.ContextLogf(ctx, "%v is not a Battery", devPath)
			continue
		}
		supplyScope, err := readLine(path.Join(devPath, "scope"))
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

// SysfsBatteryMetrics hold the metrics read from sysfs
type SysfsBatteryMetrics struct {
	powerMetric  perf.Metric
	batteryPaths []string
}

// Assert that SysfsBatteryMetrics can be used in perf.Timeline.
var _ perf.TimelineDatasource = &SysfsBatteryMetrics{}

// NewSysfsBatteryMetrics creates a struct to capture battery metrics with sysfs
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
	b.batteryPaths = batteryPaths
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
	if len(b.batteryPaths) == 0 {
		return nil
	}
	power, err := readSystemPower(b.batteryPaths)
	if err != nil {
		return err
	}
	values.Append(b.powerMetric, power)
	return nil
}
