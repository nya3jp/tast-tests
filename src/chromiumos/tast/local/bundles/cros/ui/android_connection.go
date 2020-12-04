// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/chrome/ui/nearbyshare"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AndroidConnection,
		Desc: "Connect to an Android device with adb over usb",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"nearby_snippet.apk"},
	})
}

// AndroidConnection tests that we can connect to an Android device with adb.
func AndroidConnection(ctx context.Context, s *testing.State) {
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

	// Install the snippet apk if not installed.
	// Note: Please notice the Android 11 permission issue, -g still can't grant "ALL FILE" access permission.
	pkgs, err := testDevice.InstalledPackages(ctx)
	if err != nil {
		s.Fatal("Failed to get installed packages from Android device: ", err)
	}
	if _, ok := pkgs[nearbyshare.NearbySnippetPackage]; !ok {
		if err := testDevice.Install(ctx, s.DataPath(nearbyshare.NearbySnippetApk), adb.InstallOptionGrantPermissions); err != nil {
			s.Fatal("Failed to install nearby snippet APK to device: ", err)
		}
		s.Log("Successfully install the Nearby snippet to Android device!")
	} else {
		s.Log("Skip installing the Nearby snippet to Android device, already installed!")
	}

	if err := nearbyshare.LaunchSnippet(ctx, s, testDevice); err != nil {
		s.Fatal("Failed to launch snippet on device: ", err)
	}
	defer nearbyshare.StopSnippet(ctx, s, testDevice)

}
