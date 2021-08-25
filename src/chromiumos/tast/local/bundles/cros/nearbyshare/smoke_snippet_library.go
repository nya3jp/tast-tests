// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/android/adb"
	localadb "chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SmokeSnippetLibrary,
		Desc: "Checks that we can successfully run the Nearby Snippet on the Android device",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr: []string{"group:nearby-share"},
		Data: []string{nearbysnippet.ZipName},
		// This var can be used when running locally on non-rooted devices which
		// have already overridden the GMS Core flags by other means.
		Vars: []string{"rooted"},
	})
}

// SmokeSnippetLibrary tests that we can successfully start and interact with the Nearby Snippet on the Android device.
func SmokeSnippetLibrary(ctx context.Context, s *testing.State) {
	// This loads the ARC adb vendor key, which must be pre-loaded on the Android device to allow adb over usb without requiring UI interaction.
	if err := localadb.LaunchServer(ctx); err != nil {
		s.Fatal("Failed to launch adb server: ", err)
	}

	// Wait for the first available device, since we are assuming only a single device is connected.
	testDevice, err := adb.WaitForDevice(ctx, func(device *adb.Device) bool { return true }, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to list adb devices: ", err)
	}

	// Launch and start the Snippet. Don't override GMS Core flags on a non-rooted device.
	override := true
	if val, ok := s.Var("rooted"); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Unable to convert rooted var to bool: ", err)
		}
		override = b
	}

	androidNearby, err := nearbysnippet.New(ctx, testDevice, s.DataPath(nearbysnippet.ZipName), override)
	if err != nil {
		s.Fatal("Failed to set up the snippet server: ", err)
	}
	defer androidNearby.Cleanup(ctx)

	if err := androidNearby.Initialize(ctx); err != nil {
		s.Fatal("Failed to initialize snippet server: ", err)
	}

	version, err := androidNearby.GetNearbySharingVersion(ctx)
	if err != nil {
		s.Fatal("Failed to get Android's Nearby Share version: ", err)
	}
	s.Log("Successfully got Android Nearby Share version: ", version)
}
