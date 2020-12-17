// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"strconv"

	"chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SnippetApkSmoke,
		Desc: "Checks that we can successfully run the Nearby snippet APK on the Android device",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{nearbyshare.NearbySnippetApk},
		Vars:         []string{"dontOverrideGMS"},
	})
}

// SnippetApkSmoke tests that we can successfully start and interact with the Nearby snippet APK on the Android device.
func SnippetApkSmoke(ctx context.Context, s *testing.State) {
	// This loads the ARC adb vendor key, which must be pre-loaded on the Android device to allow adb over usb without requiring UI interaction.
	if err := adb.LaunchServer(ctx); err != nil {
		s.Fatal("Failed to launch adb server: ", err)
	}

	devices, err := adb.Devices(ctx)
	if err != nil {
		s.Fatal("Failed to list adb devices: ", err)
	}
	// We assume a single device is connected.
	if len(devices) != 1 {
		s.Fatal("Unexpected number of devices: ", len(devices))
	}
	testDevice := devices[0]

	// Launch and start the snippet server. Don't override GMS Core flags if specified in the runtime vars.
	var dontOverride bool
	if val, ok := s.Var("dontOverrideGMS"); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Unable to convert dontOverrideGMS var to bool: ", err)
		}
		dontOverride = b
	} else {
		dontOverride = false
	}
	androidNearby, err := nearbyshare.PrepareAndroidNearbyDevice(ctx, testDevice, s.DataPath(nearbyshare.NearbySnippetApk), dontOverride)
	if err != nil {
		s.Fatal("Failed to set up the snippet server: ", err)
	}
	defer androidNearby.StopSnippet(ctx)

	if err := androidNearby.Initialize(); err != nil {
		s.Fatal("Failed to initialize snippet server: ", err)
	}

	version, err := androidNearby.GetNearbySharingVersion()
	if err != nil {
		s.Fatal("Failed to get Android's nearby share version: ", err)
	}
	s.Log("Successfully got Android Nearby Share version: ", version)
}
