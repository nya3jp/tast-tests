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
	"chromiumos/tast/local/bundles/cros/nearbyshare/nearbysetup"
	"chromiumos/tast/local/bundles/cros/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/bundles/cros/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosSenderAndroidReceiver,
		Desc: "Checks that we can successfully send files from a CrOS to Android",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{nearbysnippet.ZipName, "sender.js"},
		// This var can be used when running locally on non-rooted devices. For non-rooted devices, you need to
		// have already enabled bluetooth, extended the screen timeout, and overridden the GMS Core flags.
		Vars: []string{"rooted"},
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

// CrosSenderAndroidReceiver tests file sharing with a CrOS device as sender and Android device as receiver.
func CrosSenderAndroidReceiver(ctx context.Context, s *testing.State) {
	// TODO(crbug/1159975): Remove flags (or use precondition) once the feature is enabled by default.
	cr, err := chrome.New(
		ctx,
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Set up Nearby Share on the CrOS device.
	const crosBaseName = "cros_test"
	crosDisplayName := nearbytestutils.RandomDeviceName(crosBaseName)
	if err := nearbysetup.CrOSSetup(ctx, tconn, cr, nearbyshare.DataUsageOffline, nearbyshare.VisibilityAllContacts, crosDisplayName); err != nil {
		s.Fatal("Failed to set up Nearby Share: ", err)
	}

	// Set up Nearby Share on the Android device. Don't override GMS Core flags or perform settings changes that require root access if specified in the runtime vars.
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
	defer androidDevice.StopSnippet(ctx)

	// Extract the test file(s) to nearbyshare.SendDir.
	testDataZip := s.DataPath(s.Param().(nearbytestutils.TestData).Filename)
	filenames, err := nearbytestutils.ExtractCrosTestFiles(ctx, testDataZip)
	if err != nil {
		s.Fatal("Failed to extract test data files: ", err)
	}
	defer os.RemoveAll(nearbyshare.SendDir)

	// Get the full paths of the test files to pass to chrome://nearby.
	var testFiles []string
	for _, f := range filenames {
		testFiles = append(testFiles, filepath.Join(nearbyshare.SendDir, f))
	}

	// Start sending the file on the CrOS side.
	jsPath := s.DataPath("sender.js")
	sender, err := nearbyshare.StartSendFiles(ctx, cr, jsPath, testFiles)
	if err != nil {
		s.Fatal("Failed to set up control over the send surface: ", err)
	}
	defer sender.Close(ctx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Start receiving on the Android device.
	transferTimeout := s.Param().(nearbytestutils.TestData).Timeout
	callbackID, err := androidDevice.ReceiveFile(ctx, crosDisplayName, androidDisplayName, transferTimeout)
	if err != nil {
		s.Fatal("Failed to start receiving on Android: ", err)
	}
	// Defer cancelling the share in case it fails.
	defer androidDevice.CancelReceivingFile(ctx)
	// Defer taking a screenshot so we can see what receivers were available in the UI.
	// Sharing will be cancelled and Chrome will be closed before the built-in failure screenshot is taken.
	defer screenshot.CaptureChrome(ctx, cr, filepath.Join(s.OutDir(), "after_sharing.png"))

	// Wait until the Android device is detected, then select it as a receiver.
	if err := sender.WaitForShareTarget(ctx, androidDisplayName, nearbyshare.CrosDetectReceiverTimeout); err != nil {
		s.Fatal("CrOS device failed to find Android device as a receiver: ", err)
	}
	if err := sender.SelectShareTarget(ctx, androidDisplayName); err != nil {
		s.Fatal("CrOS device failed to select Android device as a receiver and start the transfer: ", err)
	}

	// Wait for Android to detect the share and start awaiting confirmation.
	if err := androidDevice.EventWaitAndGet(ctx, callbackID, nearbysnippet.SnippetEventOnLocalConfirmation, transferTimeout); err != nil {
		s.Fatal("Failed waiting for onLocalConfirmation event to know that Android is ready to start the transfer: ", err)
	}

	// Get the secure sharing token to confirm the share on Android.
	token, err := sender.ConfirmationToken(ctx)
	if err != nil {
		s.Fatal("Failed to get confirmation token: ", err)
	}

	// Confirm the share.
	if err := androidDevice.AcceptTheSharing(ctx, token); err != nil {
		s.Fatal("Failed to accept the share on the Android device: ", err)
	}

	// Check the status on CrOS until the transfer is complete.
	if err := sender.WaitForTransferStatus(ctx, nearbyshare.TransferStatusComplete, transferTimeout); err != nil {
		s.Fatal("Failed waiting for transfer to complete on CrOS: ", err)
	}

	// Wait for Android to signal the sharing has completed.
	if err := androidDevice.EventWaitAndGet(ctx, callbackID, nearbysnippet.SnippetEventOnStop, transferTimeout); err != nil {
		s.Fatal("Failed waiting for onStop to know that Android sharing has finished: ", err)
	}

	// Hash the file on both sides and confirm they match. Android receives shares in its default downloads directory.
	if err := nearbytestutils.FileHashComparison(ctx, filenames, nearbyshare.SendDir, android.DownloadDir, androidDevice); err != nil {
		s.Fatal("Failed file hash comparison: ", err)
	}
}
