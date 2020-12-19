// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/local/bundles/cros/nearbyshare/nearbysetup"
	"chromiumos/tast/local/bundles/cros/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/bundles/cros/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosToAndroidFiles,
		Desc: "Checks that we can successfully send files from a CrOS to Android",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{nearbysnippet.ZipName, "sender.js"},
		// This var can be used when running locally on non-rooted devices. For non-rooted devices, you need to
		// have already enabled bluetooth, extended the screen timeout, and overriden the GMS Core flags.
		Vars: []string{"rooted"},
		Params: []testing.Param{{
			Name:      "small_png",
			Val:       "small_png.zip",
			ExtraData: []string{"small_png.zip"},
		},
			{
				Name:      "small_jpg",
				Val:       "small_jpg.zip",
				ExtraData: []string{"small_jpg.zip"},
			}},
	})
}

// CrosToAndroidFiles tests that we can successfully start and interact with the Nearby snippet APK on the Android device.
func CrosToAndroidFiles(ctx context.Context, s *testing.State) {
	const (
		transferTimeout           = 120 * time.Second
		crosDetectReceiverTimeout = 60 * time.Second
	)
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

	// Set up the connected Android device. Don't override GMS Core flags or perform settings changes that require root change if specified in the runtime vars.
	rooted := true
	if val, ok := s.Var("rooted"); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Unable to convert rooted var to bool: ", err)
		}
		rooted = b
	}
	const androidBaseName = "ANDROID"
	androidDisplayName := nearbytestutils.RandomDeviceName(androidBaseName)
	androidDisplayName = "Pixel 3 XL" // Debug: test works on lab device with this device name, as setupDevice is not properly configuring it.
	androidDevice, err := nearbysetup.AndroidSetup(ctx, s.DataPath(nearbysnippet.ZipName), rooted, nearbysetup.DefaultScreenTimeout,
		nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityEveryone, androidDisplayName,
	)
	if err != nil {
		s.Fatal("Failed to prepare connected Android device for Nearby Share testing: ", err)
	}
	defer androidDevice.StopSnippet(ctx)
	testing.Sleep(ctx, 20*time.Second)

	// Setup the test file for sharing.
	testFilesDir := filesapp.DownloadPath
	testFiles, err := nearbytestutils.UnzipTestFiles(ctx, s.DataPath(s.Param().(string)))
	if err != nil {
		s.Fatal("Failed to extract test data files: ", err)
	}
	defer os.RemoveAll(testFilesDir)

	// Parse the sender JS data file.
	js, err := ioutil.ReadFile(s.DataPath("sender.js"))
	if err != nil {
		s.Fatal("Failed to load JS for NS sending: ", err)
	}

	// Start sending the file on the CrOS side.
	sender, err := nearbyshare.StartSendFiles(ctx, cr, string(js), testFiles)
	if err != nil {
		s.Fatal("Failed to set up control over the send surface: ", err)
	}
	defer sender.Close(ctx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Start receiving on the Android device.
	callbackID, err := androidDevice.ReceiveFile(ctx, crosDisplayName, androidDisplayName, transferTimeout)
	if err != nil {
		s.Fatal("Failed to start receiving on Android: ", err)
	}
	// Defer cancelling the share in case it fails.
	defer androidDevice.CancelReceivingFile(ctx)
	defer screenshot.CaptureChrome(ctx, cr, filepath.Join(s.OutDir(), "endreceiving.png"))

	// Debug: take a screenshot 10 seocnds after we start receiving on Android.
	testing.Sleep(ctx, 10*time.Second)
	screenshot.CaptureChrome(ctx, cr, filepath.Join(s.OutDir(), "midreceiving.png"))

	// Wait until the Android device is detected, then select it as a receiver.
	if err := sender.WaitForShareTarget(ctx, androidDisplayName, crosDetectReceiverTimeout); err != nil {
		s.Fatal("CrOS device failed to find Android device as a receiver: ", err)
	}
	if err := sender.SelectShareTarget(ctx, androidDisplayName); err != nil {
		s.Fatal("CrOs device failed to select Android device as a receiver and start the transfer: ", err)
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

	// Hash the file on both sides and confirm they match.
	if err := nearbytestutils.FileHashComparison(ctx, testFilesDir, androidDevice); err != nil {
		s.Fatal("Failed file hash comparison: ", err)
	}
}
