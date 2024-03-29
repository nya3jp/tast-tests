// Copyright 2020 The ChromiumOS Authors
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
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
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

// MapStringToBatteryStatus maps string to BatteryStatus.
func MapStringToBatteryStatus(statusStr string) (BatteryStatus, bool) {
	status, ok := batteryStatusMap[statusStr]
	if !ok {
		return BatteryStatusUnknown, false
	}
	return status, true
}

// ReadBatteryStatus returns the current battery status.
func ReadBatteryStatus(ctx context.Context, devPath string) (BatteryStatus, error) {
	statusStr, err := readFirstLine(ctx, path.Join(devPath, "status"))
	if err != nil {
		return BatteryStatusUnknown, errors.Errorf("%v lacks status attribute", devPath)
	}
	status, ok := MapStringToBatteryStatus(statusStr)
	if !ok {
		return BatteryStatusUnknown, errors.Errorf("status %v is not expected", statusStr)
	}
	return status, nil
}

// ReadBatteryCapacity returns the percentage of current charge of a battery
// which comes from /sys/class/power_supply/<supply name>/capacity.
func ReadBatteryCapacity(ctx context.Context, devPath string) (float64, error) {
	return ReadBatteryProperty(ctx, devPath, "capacity")
}

// ReadBatteryChargeNow returns the charge of a battery in Ah.
// which comes from /sys/class/power_supply/<supply name>/charge_now.
func ReadBatteryChargeNow(ctx context.Context, devPath string) (float64, error) {
	charge, err := readInt64(ctx, path.Join(devPath, "charge_now"))
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read charge from %v", devPath)
	}
	return float64(charge) / 1000000, nil
}

// WaitForCharge waits until the battery is charged.
//
// devPath - the battery to wait for.
// charge  - [0-1] how full the battery needs to be.
// timeout - The maximum time to wait.
func WaitForCharge(ctx context.Context, devPath string, charge float64, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	full, err := ReadBatteryProperty(ctx, devPath, "charge_full")
	if err != nil {
		return errors.Wrap(err, "failed to read battery charge full")
	}

	type chargeRecord struct {
		charge float64
		t      time.Time
	}
	var chargeLog []chargeRecord
	for {
		now, err := ReadBatteryProperty(ctx, devPath, "charge_now")
		if err != nil {
			return err
		}
		chargeLog = append(chargeLog, chargeRecord{charge: now, t: time.Now()})
		if now/full >= charge {
			return nil
		}
		// Fail early if there's no chance that the charge will complete before
		// the deadline.
		logLen := len(chargeLog)
		const failEarlyMinSamples = 4
		if logLen > failEarlyMinSamples {
			deltaCharge := chargeLog[logLen-1].charge - chargeLog[0].charge
			deltaT := chargeLog[logLen-1].t.Sub(chargeLog[0].t).Seconds()
			rate := deltaCharge / deltaT
			remainingT := timeout.Seconds() - deltaT
			chargeAtTimeout := now + remainingT*rate

			// Don't fail if we're within 5% of succeeding.
			const failEarlyChargeMargin = 0.05
			if chargeAtTimeout < (charge-0.05)*full {
				percentAtTimeout := 100.0 * chargeAtTimeout / full
				return errors.Errorf("charging will only restore battery to %0.2f%% before timeout", percentAtTimeout)
			}
		}
		testing.ContextLogf(ctx, "battery at %f%% < %f%%", 100.0*now/full, 100.0*charge)
		if err := testing.Sleep(ctx, 30*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for battery to charge")
		}
	}
}

// ReadBatteryEnergy returns the remaining energy of a battery in Wh.
func ReadBatteryEnergy(ctx context.Context, devPath string) (float64, error) {
	charge, err := ReadBatteryChargeNow(ctx, devPath)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read energy from %v", devPath)
	}

	voltage, err := readFloat64(ctx, path.Join(devPath, "voltage_min_design"))
	if err != nil {
		voltage, err = readFloat64(ctx, path.Join(devPath, "voltage_now"))
		if err != nil {
			return 0., errors.Wrap(err, "failed to read both voltage_min_design and voltage_now")
		}
	}
	return charge * float64(voltage) / 1000000, nil
}

// ReadSystemPower returns system power consumption in Watts.
// It is assumed that power supplies at devPath have attributes
// voltage_now and current_now.
// If reading these attributes fails, this function returns non-nil error,
// otherwise returns power consumption of the battery.
func ReadSystemPower(ctx context.Context, devPath string) (float64, error) {
	supplyVoltage, err := readFloat64(ctx, path.Join(devPath, "voltage_now"))
	if err != nil {
		return 0., errors.Wrap(err, "failed to read voltage_now")
	}
	supplyCurrent, err := readFloat64(ctx, path.Join(devPath, "current_now"))
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

// ReadBatteryProperty reads the battery property file content from the given
// battery path, and return a float value.
// The given file content should be an integer, and error will be returned otherwise.
func ReadBatteryProperty(ctx context.Context, devPath, property string) (float64, error) {
	content, err := readInt64(ctx, path.Join(devPath, property))
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read property %v from %v", property, devPath)
	}
	return float64(content), nil
}

// ListSysfsBatteryPaths lists paths of batteries which supply power to the system
// and has voltage_now and current_now attributes.
func ListSysfsBatteryPaths(ctx context.Context) ([]string, error) {
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
		supplyType, err := readFirstLine(ctx, path.Join(devPath, "type"))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read type of %v", devPath)
		}
		if supplyType != "Battery" {
			testing.ContextLogf(ctx, "%v is not a Battery", devPath)
			continue
		}
		supplyScope, err := readFirstLine(ctx, path.Join(devPath, "scope"))
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

// ErrNoBattery is an error indicating no battery is found.
var ErrNoBattery = errors.New("unexpected number of batteries: got 0; want 1")

// SysfsBatteryPath returns a path of battery which supply power to the system
// and has voltage_now and current_now attributes.
func SysfsBatteryPath(ctx context.Context) (string, error) {
	batteryPaths, err := ListSysfsBatteryPaths(ctx)
	if err != nil {
		return "", err
	}
	if len(batteryPaths) == 0 {
		return "", ErrNoBattery
	}
	if len(batteryPaths) != 1 {
		return "", errors.Errorf("unexpected number of batteries: got %d; want 1", len(batteryPaths))
	}
	return batteryPaths[0], nil
}

// SysfsBatteryMetrics hold the metrics read from sysfs.
type SysfsBatteryMetrics struct {
	powerMetric     perf.Metric
	batteryPath     string
	dischargeMetric perf.Metric
	initialEnergy   float64
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
	batteryPaths, err := ListSysfsBatteryPaths(ctx)
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
	b.initialEnergy, err = ReadBatteryEnergy(ctx, b.batteryPath)
	if err != nil {
		return err
	}
	b.powerMetric = perf.Metric{Name: prefix + "system", Unit: "W", Direction: perf.SmallerIsBetter, Multiple: true}
	b.dischargeMetric = perf.Metric{Name: prefix + "discharge_mwh", Unit: "mWh", Direction: perf.SmallerIsBetter, Multiple: false}
	return nil
}

// Start captures the initial battery state which the first snapshot will be
// relative to.
func (b *SysfsBatteryMetrics) Start(ctx context.Context) error {
	testing.ContextLog(ctx, "Start captures the initial battery state")
	return nil
}

// Snapshot takes a snapshot of battery metrics.
// If there are no batteries can be used to report the metrics,
// Snapshot does nothing and returns without error.
func (b *SysfsBatteryMetrics) Snapshot(ctx context.Context, values *perf.Values) error {
	if len(b.batteryPath) == 0 {
		return nil
	}

	power, err := ReadSystemPower(ctx, b.batteryPath)
	if err != nil {
		testing.ContextLog(ctx, "Failed to read system power: ", err)
		return err
	}

	values.Append(b.powerMetric, power)
	return nil
}

// Stop reports the total amount of energy used during the test.
func (b *SysfsBatteryMetrics) Stop(ctx context.Context, values *perf.Values) error {
	if len(b.batteryPath) == 0 {
		return nil
	}

	energy, err := ReadBatteryEnergy(ctx, b.batteryPath)
	if err != nil {
		return err
	}
	values.Set(b.dischargeMetric, 1000*(b.initialEnergy-energy))
	return nil
}
