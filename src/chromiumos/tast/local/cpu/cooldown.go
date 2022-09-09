// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cpu

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
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

// TemperatureThresholdMode defines various modes of how to decide the temperature threshold
// to cool down to.
type TemperatureThresholdMode int

const (
	// TemperatureThresholdFixed defines a fixed temperature threshold to wait
	// for.
	TemperatureThresholdFixed = iota
	// TemperatureThresholdPerModel means to check for model-specific temperature
	// thresholds, and use those. If there is no model-specific temperature
	// threshold, we use the given fixed threshold.
	TemperatureThresholdPerModel
)

// CoolDownConfig contains the config to wait for the machine to cooldown.
type CoolDownConfig struct {
	PollTimeout  time.Duration
	PollInterval time.Duration
	// TemperatureThresholdMode denotes how the temperature threshold should
	// be chosen.
	TemperatureThresholdMode TemperatureThresholdMode
	// TemperatureThreshold is the threshold for CPU temperature.
	TemperatureThreshold int
	CoolDownMode         CoolDownMode
}

// DefaultCoolDownConfig returns the default config to wait for the machine to cooldown.
func DefaultCoolDownConfig(mode CoolDownMode) CoolDownConfig {
	return CoolDownConfig{
		PollTimeout:              300 * time.Second,
		PollInterval:             2 * time.Second,
		TemperatureThresholdMode: TemperatureThresholdPerModel,
		TemperatureThreshold:     46000,
		CoolDownMode:             mode,
	}
}

// IdleCoolDownConfig returns the config to wait for the machine to cooldown for PowerIdlePerf test.
// This overrides the default config timeout (5 minutes) and temperature threshold (46 C)
// settings to reduce test flakes on low-end devices.
func IdleCoolDownConfig() CoolDownConfig {
	cdConfig := DefaultCoolDownConfig(CoolDownPreserveUI)
	cdConfig.PollTimeout = 7 * time.Minute
	cdConfig.TemperatureThreshold = 60000
	return cdConfig
}

// Temperature returns the CPU temperature in milli-Celsius units. It
// also returns the name of the thermal zone it chose.
func Temperature(ctx context.Context) (int, string, error) {
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

	zonePaths, err := filepath.Glob(thermalZonePath)
	if err != nil || len(zonePaths) == 0 {
		return 0, "", errors.Wrapf(err, "failed to glob %s", thermalZonePath)
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
			return 0, "", errors.Wrapf(err, "failed to read %q", zoneTypePath)
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
			return 0, "", errors.Wrapf(err, "failed to read %q", zoneTempPath)
		}
		zoneTemp, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil {
			return 0, "", errors.Wrapf(err, "failed to parse temperature value in %q", zoneTempPath)
		}

		return zoneTemp, zoneType, nil
	}
	return 0, "", errors.New("could not find valid thermal zone to read temperature from")
}

// WaitUntilCoolDown waits until CPU is cooled down and returns the time it took to cool down.
func WaitUntilCoolDown(ctx context.Context, config CoolDownConfig) (time.Duration, error) {
	timeBefore := time.Now()

	threshold, err := temperatureThreshold(ctx, config)
	if err != nil {
		return 0, err
	}

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
		t, z, err := Temperature(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get CPU temperature"))
		}

		if t > threshold {
			testing.ContextLogf(ctx, "Waiting until %s temperature (%d) falls below %d", z, t, threshold)
			return errors.Errorf("timed out while waiting until %s temperature (%d) falls below %d", z, t, threshold)
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

// temperatureThreshold gets the temperatuer threshold given the CoolDownConfig.
func temperatureThreshold(ctx context.Context, config CoolDownConfig) (int, error) {
	switch config.TemperatureThresholdMode {
	case TemperatureThresholdFixed:
		return config.TemperatureThreshold, nil
	case TemperatureThresholdPerModel:
		name, err := crosconfig.Get(ctx, "/", "name")
		if err != nil {
			return 0, errors.Wrap(err, "could not find model name")
		}
		if temperature, ok := modelTemperatureThresholds[name]; ok {
			testing.ContextLogf(ctx, "Using specific temperature threshold %d for model %s", temperature, name)
			return temperature, nil
		}
		testing.ContextLogf(ctx, "No per model temperature threshold, using default of %d on model %s",
			config.TemperatureThreshold, name)
		return config.TemperatureThreshold, nil
	}
	return config.TemperatureThreshold, nil
}

// modelTemperatureThresholds contains thresholds for models which we have
// determined have run particularly hot. In particular, these are set to the
// 95th percentile idle temperature for each model, as determined by the output
// of the power.IdleTemperature test. If the 95th percentile temperature is less
// than the default threshold of 46 degrees, it is not listed in here.
var modelTemperatureThresholds = map[string]int{
	"galith":     54000,
	"jelboz":     53800,
	"jelboz360":  53800,
	"berknip":    53800,
	"magneto":    53000,
	"galtic360":  53000,
	"chronicler": 53000,
	"hana":       52609,
	"metaknight": 52000,
	"pirette":    52000,
	"drawman":    52000,
	"lantis":     51000,
	"anahera":    51000,
	"pasara":     51000,
	"pirika":     51000,
	"galith360":  51000,
	"blipper":    51000,
	"ezkinil":    50800,
	"elm":        50673,
	"magma":      50000,
	"magister":   50000,
	"drawcia":    50000,
	"beetley":    50000,
	"kracko360":  50000,
	"maglith":    50000,
	"magpie":     50000,
	"drawlat":    50000,
	"dirinboz":   49800,
	"gallop":     49000,
	"lalala":     49000,
	"maglia":     49000,
	"cret":       48000,
	"magolor":    48000,
	"duffy":      48000,
	"cret360":    48000,
	"maglet":     48000,
	"eldrid":     48000,
	"gooey":      48000,
	"kracko":     48000,
	"boten":      48000,
	"sion":       48000,
	"vilboz":     47800,
	"ampton":     47000,
	"sasukette":  47000,
	"galtic":     47000,
	"bugzzy":     47000,
	"bobba360":   47000,
	"foob360":    47000,
	"galnat":     47000,
	"vilboz360":  46800,
	"vilboz14":   46800,
}
