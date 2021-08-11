// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package power provides set of util functions used to control power in ARC.
package power

import (
	"context"
	"io/ioutil"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// CoolDownMode defines various modes how to do cool down.
type CoolDownMode int

const (
	// CoolDownPreserveUI defines the mode when current Chrome UI is preserved.
	CoolDownPreserveUI CoolDownMode = iota
	// CoolDownStopUI defines the mode when current Chrome UI is stopped in order to get cool down
	// faster. However, in this mode current Chrome state is lost.
	CoolDownStopUI
)

// CoolDownConfig contains the config to wait for the machine to cooldown.
type CoolDownConfig struct {
	PollTimeout  time.Duration
	PollInterval time.Duration
	// CPUTemperatureThreshold is the threshold for CPU temperature.
	CPUTemperatureThreshold int
	CoolDownMode            CoolDownMode
}

// DefaultCoolDownConfig returns the default config to wait for the machine to cooldown.
func DefaultCoolDownConfig(mode CoolDownMode) CoolDownConfig {
	return CoolDownConfig{
		PollTimeout:             300 * time.Second,
		PollInterval:            2 * time.Second,
		CPUTemperatureThreshold: 46,
		CoolDownMode:            mode,
	}
}

// WaitUntilCPUCoolDown waits until CPU is cooled down and returns the time it
// took to cool down.
// Ported from cheets_PerfBoot.wait_cpu_cool_down().
func WaitUntilCPUCoolDown(ctx context.Context, config CoolDownConfig) (time.Duration, error) {
	timeBefore := time.Now()
	switch config.CoolDownMode {
	case CoolDownPreserveUI:
	case CoolDownStopUI:
		// Stop UI in order to cool down CPU faster as Chrome is the heaviest process when
		// system is idle.
		if err := upstart.StopJob(ctx, "ui"); err != nil {
			return 0, errors.Wrap(err, "failed to stop ui")
		}
		defer upstart.StartJob(ctx, "ui")
	default:
		return 0, errors.New("invalid cool down mode")
	}

	testing.ContextLog(ctx, "Waiting until CPU is cooled down")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		temperature, err := MaxObservedTemperature(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		if int(math.Ceil(temperature)) > config.CPUTemperatureThreshold {
			testing.ContextLogf(ctx, "Waiting until temperature (%d) falls below %d", temperature, config.CPUTemperatureThreshold)
			return errors.Errorf("timed out while waiting until temperature (%d) falls below %d", temperature, config.CPUTemperatureThreshold)
		}
		return nil
	}, &testing.PollOptions{Timeout: config.PollTimeout, Interval: config.PollInterval}); err != nil {
		return 0, err
	}

	timeAfter := time.Now()
	duration := timeAfter.Sub(timeBefore)
	testing.ContextLogf(ctx, "CPU is cooled down (took %f seconds)", duration.Seconds())
	return duration, nil
}

// MaxObservedTemperature returns the maximum observed temperature reported by sensors.
func MaxObservedTemperature(ctx context.Context) (float64, error) {
	var temperatures []float64
	// Not all temperature readings from ectool are monitoring CPU temperature. However, the cpu sensor should reports the highest temperature readings.
	if temps, err := ectoolTemperatures(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to read ectool temeprature: ", err)
	} else {
		temperatures = append(temperatures, temps...)
	}

	if temps, err := hwmonTemperatures(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to read hwmon temeprature: ", err)
	} else {
		temperatures = append(temperatures, temps...)
	}

	if temps, err := thermalZoneTemperatures(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to read thermal_zone temeprature: ", err)
	} else {
		temperatures = append(temperatures, temps...)
	}

	if len(temperatures) == 0 {
		return 0, errors.New("failed to read temperatures from all the sensors")
	}

	maxTemperature := temperatures[0]
	for _, temperature := range temperatures {
		if temperature > maxTemperature {
			maxTemperature = temperature
		}
	}
	return maxTemperature, nil
}

// ectoolTemperatures returns the temperatures in Celsius observed by ectool.
func ectoolTemperatures(ctx context.Context) ([]float64, error) {
	stdout, err := testexec.CommandContext(ctx, "ectool", "temps", "all").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run ectool")
	}

	var res []float64
	matchRegex := regexp.MustCompile(`.*: (\d+) K`)
	for _, line := range strings.Split(string(stdout), "\n") {
		matches := matchRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		temps, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse ectool output: %q", line)
		}
		// Convert to Celsius
		temperature := float64(temps - 273)
		if temperature < 10 || temperature > 150 {
			testing.ContextLog(ctx, "Unreasonable ectool temperature", temperature)
		}
		res = append(res, temperature-273)
	}
	return res, nil

}

// thermalZoneTemperatures returns the temperatures in Celsius observed by thermal_zone.
func thermalZoneTemperatures(ctx context.Context) ([]float64, error) {
	const (
		// thermalZonePath is the path to thermal zone directories.
		thermalZonePath = "/sys/class/thermal/thermal_zone*"
	)
	// thermalIgnoreTypes list thermal zones type to be ignored.
	var thermalIgnoreTypes = []string{
		// iwlwifi is the zone type of generic driver for WiFi adapters on most Intel platforms.
		"iwlwifi",
		// b/180696076: trogdor boards have a charger sensor which cools down extremely slowly when it charges the battery.
		"charger-thermal",
	}

	var res []float64
	zonePaths, err := filepath.Glob(thermalZonePath)
	if err != nil || len(zonePaths) == 0 {
		return nil, errors.Wrapf(err, "failed to glob %s", thermalZonePath)
	}
	for _, zonePath := range zonePaths {
		b, err := ioutil.ReadFile(filepath.Join(zonePath, "mode"))
		// No need to return on error because mode file doesn't always exist.
		if err == nil && strings.TrimSpace(string(b)) == "disabled" {
			continue
		}

		zoneTypePath := filepath.Join(zonePath, "type")
		b, err = ioutil.ReadFile(zoneTypePath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read %q", zoneTypePath)
		}
		zoneType := strings.TrimSpace(string(b))
		ignoreZone := false
		for _, zoneToIgnore := range thermalIgnoreTypes {
			if strings.Contains(zoneType, zoneToIgnore) {
				ignoreZone = true
				break
			}
		}
		if ignoreZone {
			continue
		}

		zoneTempPath := filepath.Join(zonePath, "temp")
		b, err = ioutil.ReadFile(zoneTempPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read %q", zoneTempPath)
		}
		zoneTemp, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse temperature value in %q", zoneTempPath)
		}

		temperature := float64(zoneTemp) / 1000
		if temperature < 10 || temperature > 150 {
			testing.ContextLogf(ctx, "Unreasonable thermal_zone(%v) temperature: %v", zoneType, temperature)
		}
		res = append(res, temperature)
	}
	return res, nil
}

// hwmonTemperatures returns the temperatures in Celsius observed by hwmon.
func hwmonTemperatures(ctx context.Context) ([]float64, error) {
	// The files contain temperature input value in millidegree Celsius.
	// https://www.kernel.org/doc/Documentation/hwmon/sysfs-interface
	const pattern = "/sys/class/hwmon/hwmon*/temp*_input"
	fs, err := filepath.Glob(pattern)
	if err != nil {
		return nil, errors.Wrap(err, "unable to obtain list of temperature files")
	}
	if len(fs) == 0 {
		return nil, errors.Errorf("no file matches %s", pattern)
	}

	var res []float64
	for _, f := range fs {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, errors.Wrap(err, "unable to read temperature file")
		}
		c, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse %s to get input temperature", f)
		}
		c = c / 1000
		if c < 10 || c > 150 {
			testing.ContextLog(ctx, "Unreasonable hwmon temperature", c)
		}
		res = append(res, c)
	}
	return res, nil
}
