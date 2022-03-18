// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	localadb "chromiumos/tast/local/android/adb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RebootPhone,
		Desc: "Reboots the connected Android device",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Attr:    []string{"group:nearby-share", "group:nearby-share-prod", "group:nearby-share-dev"},
		Timeout: 3 * time.Minute,
	})
}

// RebootPhone reboots the connected Android device. This is to help clear the phone state for other nearbyshare and crossdevice tests.
// TODO(b/207705988): Remove this test.
func RebootPhone(ctx context.Context, s *testing.State) {
	// This loads the ARC adb vendor key, which must be pre-loaded on the Android device to allow adb over usb without requiring UI interaction.
	if err := localadb.LaunchServer(ctx); err != nil {
		s.Fatal("Failed to launch adb server: ", err)
	}

	// Wait for the first available device, since we are assuming only a single device is connected.
	d, err := adb.WaitForDevice(ctx, func(device *adb.Device) bool { return !strings.HasPrefix(device.Serial, "emulator-") }, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to list adb devices: ", err)
	}

	// Reboot the device and wait for it to come up again.
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the phone: ", err)
	}
	d, err = adb.WaitForDevice(ctx, func(device *adb.Device) bool { return !strings.HasPrefix(device.Serial, "emulator-") }, time.Minute)
	if err != nil {
		s.Fatal("Failed to list adb devices: ", err)
	}

	// Wait a while to give the phone a chance to warm up so it's ready for future tests.
	testing.Sleep(ctx, time.Minute)
}
