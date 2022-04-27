// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/local/chrome/crossdevice"
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
	// Set up adb, connect to the Android phone, and check if ADB root access is available.
	adbDevice, _, err := crossdevice.AdbSetup(ctx)
	if err != nil {
		s.Fatal("Failed to set up an adb device: ", err)
	}

	// Reboot the device and wait for it to come up again.
	if err := adbDevice.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the phone: ", err)
	}

	if crossdevice.PhoneIP.Value() != "" {
		adbDevice, _, err = crossdevice.AdbSetup(ctx)
		if err != nil {
			s.Fatal("Failed to reconnect to adb device: ", err)
		}
	} else {
		adbDevice, err = adb.WaitForDevice(ctx, func(device *adb.Device) bool { return !strings.HasPrefix(device.Serial, "emulator-") }, time.Minute)
		if err != nil {
			s.Fatal("Failed to list adb devices: ", err)
		}
	}

	// Wait a while to give the phone a chance to warm up so it's ready for future tests.
	testing.Sleep(ctx, time.Minute)
}
