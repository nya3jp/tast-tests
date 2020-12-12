// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"

	"chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RunSnippetServer,
		Desc: "Set up and run simple commands on the Android Nearby snippet server",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"nearby_snippet.apk"},
	})
}

// RunSnippetServer tests that we can connect to an Android device with adb, start the snippet server apk, and get the Android Nearby version.
func RunSnippetServer(ctx context.Context, s *testing.State) {
	if err := adb.LaunchServer(ctx); err != nil {
		s.Fatal("Failed to launch adb server: ", err)
	}

	devices, err := adb.Devices(ctx)
	if err != nil {
		s.Fatal("Failed to list adb devices: ", err)
	}

	for _, device := range devices {
		s.Logf("Found device: %+v", device)
	}

	// Just one device connected for testing, otherwise can match the serial.
	testDevice := devices[0]

	// Launch and start the snippet server.
	// Note: Please notice the Android 11 permission issue, -g still can't grant "ALL FILE" access permission.
	snippet, err := nearbyshare.PrepareSnippetDevice(ctx, testDevice, s.DataPath(nearbyshare.NearbySnippetApk))
	if err != nil {
		s.Fatal("Failed to set up the snippet server: ", err)
	}
	defer snippet.StopSnippet(ctx)

	if err := snippet.Initialize(); err != nil {
		s.Fatal("Failed to initialize snippet server: ", err)
	}

	version, err := snippet.GetNearbySharingVersion()
	if err != nil {
		s.Fatal("Failed to get Android's nearby share version: ", err)
	}
	s.Log("Successfully got Android Nearby Share version: ", version)
}
