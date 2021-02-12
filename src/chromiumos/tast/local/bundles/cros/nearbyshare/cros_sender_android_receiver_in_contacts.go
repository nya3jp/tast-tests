// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"os"
	"path/filepath"
	"strconv"

	"chromiumos/tast/local/android"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysetup"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosSenderAndroidReceiverInContacts,
		Desc: "Checks that we can successfully send files between contacts from CrOS to Android",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{nearbysnippet.ZipName},
		// The rooted var can be used when running locally on non-rooted devices. For non-rooted devices, you need to
		// have already enabled bluetooth, extended the screen timeout, and overridden the GMS Core flags.
		Vars: []string{
			"rooted",
		},
		Fixture: "nearbyShareDataUsageOfflineAllContactsGAIA",
		Params: []testing.Param{
			{
				Name:      "small_png",
				Val:       nearbytestutils.TestData{Filename: "small_png.zip", Timeout: nearbyshare.SmallFileTimeout},
				ExtraData: []string{"small_png.zip"},
				Timeout:   nearbyshare.SmallFileTimeout,
			},
			{
				Name:      "small_jpg",
				Val:       nearbytestutils.TestData{Filename: "small_jpg.zip", Timeout: nearbyshare.SmallFileTimeout},
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbyshare.SmallFileTimeout,
			},
		},
	})
}

// CrosSenderAndroidReceiverInContacts tests in-contact file sharing with a CrOS device as sender and Android device as receiver.
func CrosSenderAndroidReceiverInContacts(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*nearbyshare.FixtData).Chrome
	tconn := s.FixtValue().(*nearbyshare.FixtData).TestConn
	crosDisplayName := s.FixtValue().(*nearbyshare.FixtData).DeviceName

	// Set up Nearby Share on the Android device. Don't override GMS Core flags or perform settings changes that require root access if specified in the runtime vars.
	// TODO(crbug/1171010): this test assumes the Android device is signed in as a user who is mutual contacts with the CrOS user. Add explicit Android login when available.
	rooted := true
	if val, ok := s.Var("rooted"); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Unable to convert rooted var to bool: ", err)
		}
		rooted = b
	}
	const androidBaseName = "android_test"
	androidDisplayName := nearbytestutils.RandomDeviceName(androidBaseName)
	androidDevice, err := nearbysetup.AndroidSetup(
		ctx, s.DataPath(nearbysnippet.ZipName), rooted,
		nearbysetup.DefaultScreenTimeout,
		nearbysnippet.DataUsageOffline,
		nearbysnippet.VisibilityAllContacts,
		androidDisplayName,
	)
	if err != nil {
		s.Fatal("Failed to prepare connected Android device for Nearby Share testing: ", err)
	}
	defer androidDevice.DumpLogs(ctx, s.OutDir())
	defer androidDevice.StopSnippet(ctx)

	// Extract the test file(s) to nearbytestutils.SendDir.
	testDataZip := s.DataPath(s.Param().(nearbytestutils.TestData).Filename)
	filenames, err := nearbytestutils.ExtractCrosTestFiles(ctx, testDataZip)
	if err != nil {
		s.Fatal("Failed to extract test data files: ", err)
	}
	defer os.RemoveAll(nearbytestutils.SendDir)

	// Get the full paths of the test files to pass to chrome://nearby.
	var testFiles []string
	for _, f := range filenames {
		testFiles = append(testFiles, filepath.Join(nearbytestutils.SendDir, f))
	}

	s.Log("Starting sending on the CrOS device")
	sender, err := nearbyshare.StartSendFiles(ctx, cr, testFiles)
	if err != nil {
		s.Fatal("Failed to set up control over the send surface: ", err)
	}
	defer sender.Close(ctx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Waiting for CrOS sender to detect Android receiver")
	if err := sender.SelectShareTarget(ctx, androidDisplayName, nearbyshare.CrosDetectReceiverTimeout); err != nil {
		s.Fatal("CrOS device failed to select Android device as a receiver and start the transfer: ", err)
	}

	// TODO(b/179309645): Remove UI-based Android controls once API control is available.
	s.Log("Waiting for contacts receiving UI on Android receiver")
	if err := androidDevice.InitUI(ctx); err != nil {
		s.Fatal("Failed to start UI Automator: ", err)
	}
	defer androidDevice.CloseUI(ctx)
	if err := androidDevice.WaitForInContactSenderUI(ctx, crosDisplayName, nearbyshare.CrosDetectSenderTimeout); err != nil {
		s.Fatal("Failed to find receive UI on the Android device: ", err)
	}

	s.Log("Accepting the share through the UI on the Android receiver")
	transferTimeout := s.Param().(nearbytestutils.TestData).Timeout
	if err := androidDevice.AcceptUI(ctx, transferTimeout); err != nil {
		s.Fatal("Android failed to accept the share through the UI: ", err)
	}

	// Hash the file on both sides and confirm they match. Android receives shares in its default downloads directory.
	if err := nearbytestutils.FileHashComparison(ctx, filenames, nearbytestutils.SendDir, android.DownloadDir, androidDevice); err != nil {
		s.Fatal("Failed file hash comparison: ", err)
	}
	s.Log("Share completed and file hashes match on both sides")
}
