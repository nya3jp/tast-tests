// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android"
	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbyfixture"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhoneToCrosBackgroundScanning,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that \"Nearby Device is trying to share\" notification shows up, clicking the notification enables high-vis mode and the receive flow is successful",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"hansenmichael@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:    "dataoffline_hidden_jpg11kb",
				Fixture: "nearbyShareDataUsageOfflineNoOneBackgroundScanningEnabled",
				Val: nearbycommon.TestData{
					Filename:        "small_jpg.zip",
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
					MimeType:        nearbycommon.MimeTypePng,
				},
				ExtraData:         []string{"small_jpg.zip"},
				Timeout:           nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("babymega", "barla", "blooglet", "dumo", "fennel", "soraka", "tomato", "treeya", "treeya360")),
			},
		},
	})
}

// PhoneToCrosBackgroundScanning tests background scanning and file sharing with an Android device as sender and CrOS device as receiver.
func PhoneToCrosBackgroundScanning(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*nearbyfixture.FixtData).Chrome
	tconn := s.FixtValue().(*nearbyfixture.FixtData).TestConn
	crosDisplayName := s.FixtValue().(*nearbyfixture.FixtData).CrOSDeviceName
	androidDevice := s.FixtValue().(*nearbyfixture.FixtData).AndroidDevice
	androidDisplayName := s.FixtValue().(*nearbyfixture.FixtData).AndroidDeviceName

	if err := nearbyshare.ToggleNearbyDeviceIsSharingNotification(ctx, tconn, cr /*setChecked=*/, true); err != nil {
		s.Fatal("Failed to toggle background scanning notification: ", err)
	}

	// Extract the test file to the staging directory on the Android device.
	testData := s.Param().(nearbycommon.TestData)
	testDataZip := s.DataPath(testData.Filename)
	testFile, err := nearbytestutils.ExtractAndroidTestFile(ctx, testDataZip, androidDevice)
	if err != nil {
		s.Fatal("Failed to extract test data files: ", err)
	}

	s.Log("Starting sending on the Android device")
	testTimeout := testData.TestTimeout
	mimetype := testData.MimeType
	if err := androidDevice.SendFile(ctx, androidDisplayName, crosDisplayName, testFile, mimetype, testTimeout); err != nil {
		s.Fatal("Failed to start sending on Android: ", err)
	}
	// Defer cancelling the share on the Android side if it does not succeed.
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
			if err := androidDevice.AwaitSharingStopped(ctx, testTimeout); err != nil {
				s.Error("Failed waiting for the Android device to signal that sharing has finished: ", err)
			}
		}
	}()

	s.Log("Waiting for fast init notification on CrOS receiver")
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	if err := nearbyshare.AcceptFastInitiationNotification(ctx, tconn, nearbycommon.DetectShareTargetTimeout /*isSetupComplete=*/, true); err != nil {
		s.Fatal("CrOS receiver failed to wait for Fast Init notification: ", err)
	}
	s.Log("Notification accepted. Waiting to enable High Viz Mode")

	s.Log("Getting receive surface")
	receiver, err := nearbyshare.GetReceiveSurface(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to set up control over the receiving surface: ", err)
	}
	defer receiver.Close(ctx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Waiting for CrOS receiver to detect incoming share from Android sender")
	crosToken, err := receiver.WaitForSender(ctx, androidDisplayName, nearbycommon.DetectShareTargetTimeout)
	if err != nil {
		s.Fatal("CrOS receiver failed to find Android sender: ", err)
	}

	s.Log("Waiting for Android sender to see that CrOS receiver connected")
	androidToken, err := androidDevice.AwaitReceiverAccept(ctx, nearbycommon.DetectShareTargetTimeout)
	if err != nil {
		s.Fatal("Failed waiting for the Android to connect to receiver: ", err)
	}
	if crosToken != androidToken {
		s.Fatalf("Share tokens for Android and CrOS do not match. Android: %s, CrOS: %s", androidToken, crosToken)
	}
	s.Log("Accepting the share on the CrOS receiver")
	if err := receiver.AcceptShare(ctx); err != nil {
		s.Fatal("CrOs receiver failed to accept share from Android sender: ", err)
	}

	s.Log("Waiting for the Android sender to signal that sharing has completed")
	if err := androidDevice.AwaitSharingStopped(ctx, testData.TransferTimeout); err != nil {
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
