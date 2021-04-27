// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package power provides set of util functions used to control power in ARC.
package power

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
	// CPUTemperatureThreshold is the threshold for CPU temperature.
	CPUTemperatureThreshold int
	CoolDownMode            CoolDownMode
}

// DefaultCoolDownConfig returns the default config to wait for the machine to cooldown.
func DefaultCoolDownConfig(mode CoolDownMode) CoolDownConfig {
	return CoolDownConfig{PollTimeout: 300 * time.Second, PollInterval: 2 * time.Second, CPUTemperatureThreshold: 46000, CoolDownMode: mode}
}

// WaitUntilCPUCoolDown waits until CPU is cooled down and returns the time it
// took to cool down.
// Ported from cheets_PerfBoot.wait_cpu_cool_down().
func WaitUntilCPUCoolDown(ctx context.Context, config CoolDownConfig) (time.Duration, error) {
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

	zonePaths, err := filepath.Glob(thermalZonePath)
	if err != nil || len(zonePaths) == 0 {
		return 0, errors.Wrapf(err, "failed to glob %s", thermalZonePath)
	}

	testing.ContextLog(ctx, "Waiting until CPU is cooled down")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		for _, zonePath := range zonePaths {
			b, err := ioutil.ReadFile(filepath.Join(zonePath, "mode"))
			// No need to return on error because mode file doesn't always exist.
			if err == nil && strings.TrimSpace(string(b)) == "disabled" {
				continue
			}

			zoneTypePath := filepath.Join(zonePath, "type")
			b, err = ioutil.ReadFile(zoneTypePath)
			if err != nil {
				return testing.PollBreak(
					errors.Wrapf(err, "failed to read %q", zoneTypePath))
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
				return testing.PollBreak(
					errors.Wrapf(err, "failed to read %q", zoneTempPath))
			}
			zoneTemp, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err != nil {
				return testing.PollBreak(errors.Wrapf(err,
					"failed to parse temperature value in %q", zoneTempPath))
			}

			if zoneTemp > config.CPUTemperatureThreshold {
				testing.ContextLogf(ctx, "Waiting until %s temperature (%d) falls below %d",
					zoneType, zoneTemp, config.CPUTemperatureThreshold)
				return errors.Errorf("timed out while waiting until %s temperature (%d) falls below %d",
					zoneType, zoneTemp, config.CPUTemperatureThreshold)
			}
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
