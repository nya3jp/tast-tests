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

// WaitUntilCPUCoolDown waits until CPU is cooled down.
// Ported from cheets_PerfBoot.wait_cpu_cool_down().
func WaitUntilCPUCoolDown(ctx context.Context, coolDownMode CoolDownMode) error {
	const (
		pollTimeout  = 300 * time.Second
		pollInterval = 3 * time.Second

		// cpuTemperatureThreshold is the threshold for CPU temperature.
		cpuTemperatureThreshold = 46000

		// thermalZonePath is the path to thermal zone directories.
		thermalZonePath = "/sys/class/thermal/thermal_zone*"

		// thermalIgnoreType is a thermal zone type to be ignored.
		// iwlwifi is the zone type of generic driver for WiFi adapters on most Intel platforms.
		thermalIgnoreType = "iwlwifi"
	)

	switch coolDownMode {
	case CoolDownPreserveUI:
	case CoolDownStopUI:
		// Stop UI in order to cool down CPU faster as Chrome is the heaviest process when
		// system is idle.
		if err := upstart.StopJob(ctx, "ui"); err != nil {
			return errors.Wrap(err, "failed to stop ui")
		}
		defer upstart.StartJob(ctx, "ui")
	default:
		return errors.New("invalid cool down mode")
	}

	zonePaths, err := filepath.Glob(thermalZonePath)
	if err != nil || len(zonePaths) == 0 {
		return errors.Wrapf(err, "failed to glob %s", thermalZonePath)
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
			if zoneType == thermalIgnoreType {
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

			if zoneTemp > cpuTemperatureThreshold {
				testing.ContextLogf(ctx, "Waiting until %s temperature (%d) falls below %d",
					zoneType, zoneTemp, cpuTemperatureThreshold)
				return errors.Errorf(
					"timed out while waiting until %s temperature (%d) falls below %d",
					zoneType, zoneTemp, cpuTemperatureThreshold)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: pollTimeout, Interval: pollInterval}); err != nil {
		return err
	}

	testing.ContextLog(ctx, "CPU is cooled down")
	return nil
}
