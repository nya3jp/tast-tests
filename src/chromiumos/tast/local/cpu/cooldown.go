// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
	// TemperatureThreshold is the threshold for CPU temperature.
	TemperatureThreshold int
	CoolDownMode         CoolDownMode
}

// DefaultCoolDownConfig returns the default config to wait for the machine to cooldown.
func DefaultCoolDownConfig(mode CoolDownMode) CoolDownConfig {
	return CoolDownConfig{
		PollTimeout:          300 * time.Second,
		PollInterval:         2 * time.Second,
		TemperatureThreshold: 46000,
		CoolDownMode:         mode,
	}
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

		if t > config.TemperatureThreshold {
			testing.ContextLogf(ctx, "Waiting until %s temperature (%d) falls below %d", z, t, config.TemperatureThreshold)
			return errors.Errorf("timed out while waiting until %s temperature (%d) falls below %d", z, t, config.TemperatureThreshold)
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
