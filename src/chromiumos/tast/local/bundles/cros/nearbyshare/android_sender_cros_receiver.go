// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
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
		// TODO(crbug/1127165) Move to fixture when data is available.
		Data: []string{nearbysnippet.ZipName},
		// This var can be used when running locally on non-rooted devices. For non-rooted devices, you need to
		// have already enabled bluetooth, extended the screen timeout, and overridden the GMS Core flags.
		Vars:    []string{"rooted"},
		Fixture: "nearbyShareDataUsageOfflineAllContactsTestUser",
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
	cr := s.FixtValue().(*nearbyshare.FixtData).Chrome
	tconn := s.FixtValue().(*nearbyshare.FixtData).TestConn
	crosDisplayName := s.FixtValue().(*nearbyshare.FixtData).CrOSDeviceName
	androidDevice := s.FixtValue().(*nearbyshare.FixtData).AndroidDevice
	androidDisplayName := s.FixtValue().(*nearbyshare.FixtData).AndroidDeviceName
	defer androidDevice.DumpLogs(ctx, s.OutDir())

	// Extract the test file to the staging directory on the Android device.
	testData := s.Param().(nearbytestutils.TestData)
	testDataZip := s.DataPath(testData.Filename)
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
	transferTimeout := testData.Timeout
	mimetype := testData.MimeType
	if err := androidDevice.SendFile(ctx, androidDisplayName, crosDisplayName, testFile, mimetype, transferTimeout); err != nil {
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
