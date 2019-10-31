// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sysutil

import (
	"context"
	"io/ioutil"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// TemperatureInputMax returns the maximum currently observed temperature in Celsius.
func TemperatureInputMax() (float64, error) {
	// The files contain temperature input value in millidegree Celsius.
	// https://www.kernel.org/doc/Documentation/hwmon/sysfs-interface
	const pattern = "/sys/class/hwmon/hwmon*/temp*_input"
	fs, err := filepath.Glob(pattern)
	if err != nil {
		return 0, errors.Wrap(err, "unable to obtain list of temperature files")
	}
	if len(fs) == 0 {
		return 0, errors.Errorf("no file matches %s", pattern)
	}

	res := math.Inf(-1)
	for _, f := range fs {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return 0, errors.Wrap(err, "unable to read temperature file")
		}
		c, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil {
			return 0, errors.Wrapf(err, "could not parse %s to get input temperature", f)
		}
		res = math.Max(res, c/1000)
	}
	return res, nil
}

// TemperatureCritical returns the temperature at which we will see some throttling in the system in Celcius.
// It returns a sensible default if the observed value isn't in the normal range due to a bug in the board.
func TemperatureCritical(ctx context.Context) (float64, error) {
	// The files contain critical temperature max value in millidegree Celsius.
	// https://www.kernel.org/doc/Documentation/hwmon/sysfs-interface
	const pattern = "/sys/class/hwmon/hwmon*/temp*_crit"
	fs, err := filepath.Glob(pattern)
	if err != nil {
		return 0, errors.Wrap(err, "unable to obtain list of temperature files")
	}
	if len(fs) == 0 {
		return 0, errors.Errorf("no file matches %s", pattern)
	}

	// Compute the minimum value among all.
	res := math.Inf(1)
	for _, f := range fs {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return 0, errors.Wrap(err, "unable to read temperature file")
		}
		c, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil {
			return 0, errors.Wrapf(err, "could not parse %s to get critical temperature", f)
		}
		// Files can show 0 on certain boards (crbug.com/360249).
		if c == 0 {
			continue
		}
		res = math.Min(res, c/1000)
	}
	if res < 60 || 150 < res {
		// Got suspicious result; use typical value for the machine.
		var typical float64
		u, err := Uname()
		if err != nil {
			return 0, err
		}
		// Today typical for Intel is 98'C to 105'C while ARM is 85'C. Clamp to 98
		// if Intel device or the lowest known value otherwise (crbug.com/360249).
		if strings.Contains(u.Machine, "x86") {
			typical = 98
		} else {
			typical = 85
		}
		testing.ContextLogf(ctx, "Computed critical temperature %.1fC is suspicious; returning %.1fC", res, typical)
		return typical, nil
	}
	return res, nil
}
