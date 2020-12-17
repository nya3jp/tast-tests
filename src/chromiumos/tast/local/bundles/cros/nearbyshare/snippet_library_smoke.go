// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"strconv"

	"chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/bundles/cros/nearbyshare/nearbysnippet"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SnippetLibrarySmoke,
		Desc: "Checks that we can successfully run the Nearby Snippet on the Android device",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{nearbysnippet.ZipName},
		// This var can be used when running locally on non-rooted devices which
		// have already overridden the GMS Core flags by other means.
		Vars: []string{"dontOverrideGMS"},
	})
}

// SnippetLibrarySmoke tests that we can successfully start and interact with the Nearby Snippet on the Android device.
func SnippetLibrarySmoke(ctx context.Context, s *testing.State) {
	// This loads the ARC adb vendor key, which must be pre-loaded on the Android device to allow adb over usb without requiring UI interaction.
	if err := adb.LaunchServer(ctx); err != nil {
		s.Fatal("Failed to launch adb server: ", err)
	}

	devices, err := adb.Devices(ctx)
	if err != nil {
		s.Fatal("Failed to list adb devices: ", err)
	}
	if len(devices) != 1 {
		// TODO(crbug/1159996): Skip running this test if the DUT doesn't have a phone connected.
		s.Fatal("Failed to find a connected Android device")
	}
	// We assume a single device is connected.
	testDevice := devices[0]

	// Launch and start the Snippet. Don't override GMS Core flags if specified in the runtime vars.
	var dontOverride bool
	if val, ok := s.Var("dontOverrideGMS"); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Unable to convert dontOverrideGMS var to bool: ", err)
		}
		dontOverride = b
	}
	androidNearby, err := nearbysnippet.New(ctx, testDevice, s.DataPath(nearbysnippet.ZipName), dontOverride)
	if err != nil {
		s.Fatal("Failed to set up the snippet server: ", err)
	}
	defer androidNearby.StopSnippet(ctx)

	if err := androidNearby.Initialize(); err != nil {
		s.Fatal("Failed to initialize snippet server: ", err)
	}

	version, err := androidNearby.GetNearbySharingVersion()
	if err != nil {
		s.Fatal("Failed to get Android's Nearby Share version: ", err)
	}
	s.Log("Successfully got Android Nearby Share version: ", version)
}
