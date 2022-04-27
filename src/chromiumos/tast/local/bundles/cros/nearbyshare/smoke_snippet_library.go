// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/crossdevice"
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
		Attr:    []string{"group:nearby-share"},
		Data:    []string{nearbysnippet.ZipName},
		Timeout: 3 * time.Minute,
	})
}

// SmokeSnippetLibrary tests that we can successfully start and interact with the Nearby Snippet on the Android device.
func SmokeSnippetLibrary(ctx context.Context, s *testing.State) {
	// Set up adb, connect to the Android phone, and check if ADB root access is available.
	adbDevice, rooted, err := crossdevice.AdbSetup(ctx)
	if err != nil {
		s.Fatal("Failed to set up an adb device: ", err)
	}

	androidNearby, err := nearbysnippet.New(ctx, adbDevice, s.DataPath(nearbysnippet.ZipName), rooted)
	if err != nil {
		s.Fatal("Failed to set up the snippet server: ", err)
	}
	defer androidNearby.Cleanup(ctx)

	version, err := androidNearby.GetNearbySharingVersion(ctx)
	if err != nil {
		s.Fatal("Failed to get Android's Nearby Share version: ", err)
	}
	s.Log("Successfully got Android Nearby Share version: ", version)
}
