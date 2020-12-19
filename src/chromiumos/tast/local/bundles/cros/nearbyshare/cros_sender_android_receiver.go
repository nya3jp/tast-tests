// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/android/nearbysnippet"
	"chromiumos/tast/local/bundles/cros/nearbyshare/nearbysetup"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/ui/filesapp"
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
		Data:         []string{nearbysnippet.NearbySnippetZip, "small_image.png", "sender.js"},
		// This var can be used when running locally on non-rooted devices which
		// have already overriden the GMS Core flags by other means.
		Vars: []string{"dontOverrideGMS"},
	})
}

// CrosSenderAndroidReceiver tests that we can successfully start and interact with the Nearby snippet APK on the Android device.
func CrosSenderAndroidReceiver(ctx context.Context, s *testing.State) {
	const (
		transferTimeout           = 120 * time.Second
		crosDetectReceiverTimeout = 20 * time.Second
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
	const crosName = "cros_test"
	if err := nearbysetup.CrOSSetup(ctx, tconn, cr, nearbyshare.DataUsageOffline, nearbyshare.VisibilityAllContacts, crosName); err != nil {
		s.Fatal("Failed to set up Nearby Share: ", err)
	}

	// Set up the connected Android device. Don't override GMS Core flags if specified in the runtime vars.
	dontOverride := false
	if val, ok := s.Var("dontOverrideGMS"); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Unable to convert dontOverrideGMS var to bool: ", err)
		}
		dontOverride = b
	}
	const androidName = "android_test"
	androidDevice, err := nearbysetup.AndroidSetup(ctx, s.DataPath(nearbysnippet.NearbySnippetZip), dontOverride, nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityEveryone, androidName)
	if err != nil {
		s.Fatal("Failed to prepare connected Android device for Nearby Share testing: ", err)
	}
	defer androidDevice.StopSnippet(ctx)

	// Setup the test file for sharing.
	const testFileName = "small_image.png"
	testFilePath := filepath.Join(filesapp.DownloadPath, testFileName)
	if err := fsutil.CopyFile(s.DataPath(testFileName), testFilePath); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", testFilePath, err)
	}
	defer os.Remove(testFilePath)

	// Parse the sender JS data file.
	js, err := ioutil.ReadFile(s.DataPath("sender.js"))
	if err != nil {
		s.Fatal("Failed to load JS for NS sending: ", err)
	}

	// Start sending the file on the CrOS side.
	sender, err := nearbyshare.StartSendFiles(ctx, cr, string(js), []string{testFilePath})
	if err != nil {
		s.Fatal("Failed to set up control over the send surface: ", err)
	}
	defer sender.Close()

	// Start receiving on the Android device.
	callbackID, err := androidDevice.ReceiveFile(crosName, androidName, transferTimeout)
	if err != nil {
		s.Fatal("Failed to start receiving on Android: ", err)
	}
	// Defer cancelling the share in case it fails.
	defer androidDevice.CancelReceivingFile()

	// Wait until the Android device is detected, then select it as a receiver.
	if err := sender.WaitForShareTarget(ctx, androidName, crosDetectReceiverTimeout); err != nil {
		s.Fatal("CrOS device failed to find Android device as a receiver: ", err)
	}
	if err := sender.SelectShareTarget(ctx, androidName); err != nil {
		s.Fatal("CrOs device failed to select Android device as a receiver and start the transfer: ", err)
	}

	// Wait for Android to detect the share and start awaiting confirmation.
	if err := androidDevice.EventWaitAndGet(callbackID, nearbysnippet.SnippetEventOnLocalConfirmation, transferTimeout); err != nil {
		s.Fatal("Failed waiting for onLocalConfirmation event to know that Android is ready to start the transfer: ", err)
	}

	// Get the secure sharing token to confirm the share on Android.
	token, err := sender.ConfirmationToken(ctx)
	if err != nil {
		s.Fatal("Failed to get confirmation token: ", err)
	}

	// Confirm the share.
	if err := androidDevice.AcceptTheSharing(token); err != nil {
		s.Fatal("Failed to accept the share on the Android device: ", err)
	}

	// Check the status on CrOS until the transfer is complete.
	if err := sender.WaitForTransferStatus(ctx, nearbyshare.TransferStatusComplete, transferTimeout); err != nil {
		s.Fatal("Failed waiting for transfer to complete on CrOS: ", err)
	}

	// Wait for Android to signal the sharing has completed.
	if err := androidDevice.EventWaitAndGet(callbackID, nearbysnippet.SnippetEventOnStop, transferTimeout); err != nil {
		s.Fatal("Failed waiting for onStop to know that Android sharing has finished: ", err)
	}

	// Hash the file on both sides and confirm they match.
	f, err := os.Open(testFilePath)
	if err != nil {
		s.Fatal("Failed to open test file on CrOS: ", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		s.Fatal("Failed to copy file contents to the hasher: ", err)
	}
	crosHash := hex.EncodeToString(h.Sum(nil))

	androidHash, err := androidDevice.HashFile(ctx, testFileName)
	if err != nil {
		s.Fatal("Failed to get test file's sha256sum on Android: ", err)
	}
	s.Log("crosHash: ", crosHash)
	s.Log("androidHash: ", androidHash)

	if crosHash != androidHash {
		s.Fatal("File hashes did not match; CrOS: %v; Android: %v", crosHash, androidHash)
	}
}
