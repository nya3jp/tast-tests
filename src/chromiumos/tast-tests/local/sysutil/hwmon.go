// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sysutil

import (
	"io/ioutil"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
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
