// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/testexec"
	localadb "chromiumos/tast/local/android/adb"
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
	// This loads the ARC adb vendor key, which must be pre-loaded on the Android device to allow adb over usb without requiring UI interaction.
	if err := localadb.LaunchServer(ctx); err != nil {
		s.Fatal("Failed to launch adb server: ", err)
	}

	var adbDevice *adb.Device
	var err error
	PhoneIP := crossdevice.PhoneIP
	if PhoneIP.Value() != "" {
		testing.ContextLogf(ctx, "Android phone IP is: %s", PhoneIP.Value())

		// Ensure CrOS device is on the correct Wifi network
		out, err := testexec.CommandContext(ctx, "/usr/local/autotest/cros/scripts/wifi", "connect", "nearbysharing_1", "password").CombinedOutput(testexec.DumpLogOnError)
		if err != nil {
			if strings.Contains(string(out), "already connected") {
				s.Log("Already connected to wifi network")
			} else {
				s.Fatal("Failed to connect CrOS device to Wifi: ", err)
			}
		}

		// Connect to the adb-over-tcp Phone that was setup previously (e.g manually or via autotest control file).
		adbDevice, err = adb.Connect(ctx, PhoneIP.Value(), 30*time.Second)
		if err != nil {
			s.Fatal("Failed to connect to adb over wifi: ", err)
		}
		s.Log("Connected to remote Android device")

		// Wait for the Android device to be ready for use.
		if err := adbDevice.WaitForState(ctx, adb.StateDevice, 30*time.Second); err != nil {
			s.Fatal("Waiting for adb state failed: ", err)
		}
		// Reboot the device and wait for it to come up again.
		if err := adbDevice.Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot the phone: ", err)
		}
		adbDevice, err = adb.Connect(ctx, PhoneIP.Value(), 1*time.Minute)
		if err != nil {
			s.Fatal("Failed to connect to adb over wifi: ", err)
		}
		s.Log("Reconnected to remote Android device after reboot")
		// Wait for the Android device to be ready for use.
		if err := adbDevice.WaitForState(ctx, adb.StateDevice, 30*time.Second); err != nil {
			s.Fatal("Wait for android state failed: ", err)
		}
	} else {
		// Wait for the first available device, since we are assuming only a single device is connected.
		adbDevice, err = adb.WaitForDevice(ctx, func(device *adb.Device) bool { return !strings.HasPrefix(device.Serial, "emulator-") }, 10*time.Second)
		if err != nil {
			s.Fatal("Failed to list adb devices: ", err)
		}
		// Reboot the device and wait for it to come up again.
		if err := adbDevice.Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot the phone: ", err)
		}
		adbDevice, err = adb.WaitForDevice(ctx, func(device *adb.Device) bool { return !strings.HasPrefix(device.Serial, "emulator-") }, time.Minute)
		if err != nil {
			s.Fatal("Failed to list adb devices: ", err)
		}
	}

	// Wait a while to give the phone a chance to warm up so it's ready for future tests.
	testing.Sleep(ctx, time.Minute)
}
