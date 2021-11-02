// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/errors"
	localadb "chromiumos/tast/local/android/adb"
	"chromiumos/tast/testing"
)

// AdbSetup configures adb and connects to the Android device with adb root if available.
func AdbSetup(ctx context.Context) (*adb.Device, bool, error) {
	// Load the ARC adb vendor key, which must be pre-loaded on the Android device to allow adb over usb without requiring UI interaction.
	if err := localadb.LaunchServer(ctx); err != nil {
		return nil, false, errors.Wrap(err, "failed to launch adb server")
	}
	// Wait for the first available device, since we are assuming only a single Android device is connected to each CrOS device.
	adbDevice, err := adb.WaitForDevice(ctx, func(device *adb.Device) bool { return !strings.HasPrefix(device.Serial, "emulator-") }, 10*time.Second)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to list adb devices")
	}
	// Check if adb root is available.
	rooted := true
	if err := adbDevice.Root(ctx); err != nil {
		testing.ContextLog(ctx, "ADB root access not available; operations requiring root access will be skipped")
		rooted = false
	}
	return adbDevice, rooted, nil
}
