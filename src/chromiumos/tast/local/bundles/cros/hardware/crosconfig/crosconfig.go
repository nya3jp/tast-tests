// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosconfig provides methods for interacting with the cros_config
// command line utility. See https://chromium.googlesource.com/chromiumos/platform2/+/HEAD/chromeos-config
// for more information about cros_config.
package crosconfig

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// HardwareProperty represents an attribute in /hardware-properties.
// https://chromium.googlesource.com/chromiumos/platform2/+/HEAD/chromeos-config#hardware_properties
type HardwareProperty string

const (
	// HasBaseAccelerometer is a bool property describing whether the DUT has an
	// accelerometer in its base.
	HasBaseAccelerometer HardwareProperty = "has-base-accelerometer"
	// HasBaseGyroscope is a bool property describing whether the DUT has an
	// gyroscope in its base.
	HasBaseGyroscope HardwareProperty = "has-base-gyroscope"
	// HasBaseMagnetometer is a bool property describing whether the DUT has an
	// magnetometer in its base.
	HasBaseMagnetometer HardwareProperty = "has-base-magnetometer"
	// HasLidAccelerometer is a bool property describing whether the DUT has an
	// accelerometer in its lid.
	HasLidAccelerometer HardwareProperty = "has-lid-accelerometer"
	// HasLidGyroscope is a bool property describing whether the DUT has an
	// gyroscope in its lid.
	HasLidGyroscope HardwareProperty = "has-lid-gyroscope"
	// HasLidMagnetometer is a bool property describing whether the DUT has an
	// magnetometer in its lid.
	HasLidMagnetometer HardwareProperty = "has-lid-magnetometer"
)

var runCrosConfig = func(ctx context.Context, args ...string) ([]byte, error) {
	return testexec.CommandContext(ctx, "cros_config", args...).Output()
}

// CheckHardwareProperty returns true if the given hardware property is set to true and
// it returns false if the property is set to false or not set.
func CheckHardwareProperty(ctx context.Context, prop HardwareProperty) (bool, error) {
	output, err := runCrosConfig(ctx, "/hardware-properties", string(prop))
	status, ok := testexec.GetWaitStatus(err)

	if !ok {
		return false, err
	}

	switch status.ExitStatus() {
	case 0:
		val := strings.TrimSpace(string(output))
		switch val {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return false, errors.Errorf("unknown output %q", val)
		}
	case 1:
		// If cros_config exits with a code of 1 it means that the value was not
		// present in the model.yaml.
		return false, nil
	default:
		return false, err
	}
}
