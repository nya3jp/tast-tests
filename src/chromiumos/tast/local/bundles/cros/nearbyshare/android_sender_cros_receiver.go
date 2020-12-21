// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android"
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
		Func: AndroidSenderCrosReceiver,
		Desc: "Checks that we can successfully send files from Android to CrOS",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{nearbysnippet.ZipName},
		// This var can be used when running locally on non-rooted devices. For non-rooted devices, you need to
		// have already enabled bluetooth, extended the screen timeout, and overridden the GMS Core flags.
		Vars: []string{"rooted"},
		Params: []testing.Param{
			{
				Name: "small_jpg",
				Val: nearbytestutils.TestData{
					Filename: "small_jpg.zip",
					Timeout:  nearbyshare.SmallFileTimeout,
					MimeType: nearbysnippet.MimeTypeJpeg,
				},
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbyshare.SmallFileTimeout,
			},
		},
	})
}

// AndroidSenderCrosReceiver tests file sharing with an Android device as sender and CrOS device as receiver.
func AndroidSenderCrosReceiver(ctx context.Context, s *testing.State) {
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

	// Extract the test file to the staging directory on the Android device.
	testDataZip := s.DataPath(s.Param().(nearbytestutils.TestData).Filename)
	testFile, err := nearbytestutils.ExtractAndroidTestFile(ctx, testDataZip, androidDevice)
	if err != nil {
		s.Fatal("Failed to extract test data files: ", err)
	}

	s.Log("Starting receiving on the CrOS device")
	receiver, err := nearbyshare.StartReceiving(ctx, cr)
	if err != nil {
		s.Fatal("Failed to set up control over the receiving surface: ", err)
	}
	defer receiver.Close(ctx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Starting sending on the Android device")
	transferTimeout := s.Param().(nearbytestutils.TestData).Timeout
	mimetype := s.Param().(nearbytestutils.TestData).MimeType
	if err := androidDevice.SendFile(ctx, androidDisplayName, crosDisplayName, testFile, mimetype, transferTimeout); err != nil {
		s.Fatal("Failed to start sending on Android: ", err)
	}
	// Defer cancelling the share in case it fails.
	var shareCompleted bool
	defer func() {
		if !shareCompleted {
			s.Log("Cancelling sending")
			if err := screenshot.CaptureChrome(ctx, cr, filepath.Join(s.OutDir(), "after_sharing.png")); err != nil {
				s.Log("Failed to capture a screenshot before cancelling sending")
			}
			if err := androidDevice.CancelSendingFile(ctx); err != nil {
				s.Error("Failed to cancel sending after the share failed: ", err)
			}
			if err := androidDevice.AwaitSharingStopped(ctx, transferTimeout); err != nil {
				s.Error("Failed waiting for the Android device to signal that sharing has finished: ", err)
			}
		}
	}()

	s.Log("Waiting for CrOS receiver to detect incoming share from Android sender")
	if _, err := receiver.WaitForSender(ctx, androidDisplayName, nearbyshare.CrosDetectSenderTimeout); err != nil {
		s.Fatal("CrOS receiver failed to find Android sender: ", err)
	}
	s.Log("Accepting the share on the CrOS receiver")
	if err := receiver.AcceptShare(ctx); err != nil {
		s.Fatal("CrOs receiver failed to accept share from Android sender: ", err)
	}

	s.Log("Waiting for the Android sender to signal that sharing has completed")
	if err := androidDevice.AwaitSharingStopped(ctx, transferTimeout); err != nil {
		s.Fatal("Failed waiting for the Android device to signal that sharing has finished: ", err)
	}
	shareCompleted = true

	// Repeat the file hash check for a few seconds, as we have no indicator on the CrOS side for when the received file has been completely written.
	// TODO(crbug/1173190): Remove polling when we can confirm the transfer status with public test functions.
	s.Log("Comparing Android and CrOS file hashes")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := nearbytestutils.FileHashComparison(ctx, []string{testFile}, filesapp.DownloadPath, filepath.Join(android.DownloadDir, nearbysnippet.SendDir), androidDevice); err != nil {
			return errors.Wrap(err, "file hashes don't match yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed file hash comparison: ", err)
	}
	s.Log("Share completed and file hashes match on both sides")
}
