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

	"chromiumos/tast/local/testexec"
)

// HardwareProperty represents an attribute in /hardware-properties
// https://chromium.googlesource.com/chromiumos/platform2/+/HEAD/chromeos-config#hardware_properties
type HardwareProperty string

const (
	// HasBaseAccelerometer does the dut have an accelerometer in its base
	HasBaseAccelerometer HardwareProperty = "has-base-accelerometer"
	// HasBaseGyroscope does the dut have a gyroscope in its base
	HasBaseGyroscope HardwareProperty = "has-base-gyroscope"
	// HasBaseMagnetometer does the dut have a gyroscope in its base
	HasBaseMagnetometer HardwareProperty = "has-base-magnetometer"
	// HasLidAccelerometer does the dut have an accelerometer in its lid
	HasLidAccelerometer HardwareProperty = "has-lid-accelerometer"
)

var runCrosConfig = func(ctx context.Context, arg ...string) ([]byte, error) {
	cmd := testexec.CommandContext(ctx, "cros_config", arg...)
	return cmd.Output()
}

// CheckHardwareProperty returns true if the given hardware property is set to true and
// it returns false if the property is set to false or not set.
func CheckHardwareProperty(ctx context.Context, prop HardwareProperty) (bool, error) {
	output, err := runCrosConfig(ctx, "/hardware-properties", string(prop))
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(string(output)) == "true", nil
}
