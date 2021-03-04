// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/android"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosToPhoneHighVis,
		Desc: "Checks that we can successfully send files from a CrOS to Android",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		// TODO(crbug/1127165) Move to fixture when data is available.
		Data:    []string{nearbysnippet.ZipName, nearbysnippet.AccountUtilZip},
		Fixture: "nearbyShareDataUsageOfflineAllContactsTestUser",
		Params: []testing.Param{
			{
				Name: "dataoffline_allcontacts_png5kb",
				Val: nearbytestutils.TestData{
					Filename:        "small_png.zip",
					TransferTimeout: nearbyshare.SmallFileTransferTimeout,
					TestTimeout:     nearbyshare.DetectionTimeout + nearbyshare.SmallFileTransferTimeout,
				},
				ExtraData: []string{"small_png.zip"},
				Timeout:   nearbyshare.DetectionTimeout + nearbyshare.SmallFileTransferTimeout,
			},
			{
				Name: "dataoffline_allcontacts_jpg11kb",
				Val: nearbytestutils.TestData{
					Filename:        "small_jpg.zip",
					TransferTimeout: nearbyshare.SmallFileTransferTimeout,
					TestTimeout:     nearbyshare.DetectionTimeout + nearbyshare.SmallFileTransferTimeout,
				},
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbyshare.DetectionTimeout + nearbyshare.SmallFileTransferTimeout,
			},
		},
	})
}

// CrosToPhoneHighVis tests file sharing with a CrOS device as sender and Android device as receiver.
func CrosToPhoneHighVis(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*nearbyshare.FixtData).Chrome
	tconn := s.FixtValue().(*nearbyshare.FixtData).TestConn
	crosDisplayName := s.FixtValue().(*nearbyshare.FixtData).CrOSDeviceName
	androidDevice := s.FixtValue().(*nearbyshare.FixtData).AndroidDevice
	androidDisplayName := s.FixtValue().(*nearbyshare.FixtData).AndroidDeviceName

	// Extract the test file(s) to nearbyshare.SendDir.
	testData := s.Param().(nearbytestutils.TestData)
	testDataZip := s.DataPath(testData.Filename)
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

	s.Log("Starting receiving on the Android device")
	testTimeout := testData.TestTimeout
	if err := androidDevice.ReceiveFile(ctx, crosDisplayName, androidDisplayName, testTimeout); err != nil {
		s.Fatal("Failed to start receiving on Android: ", err)
	}
	// Defer cancelling receiving if something goes wrong.
	var shareCompleted bool
	defer func() {
		if !shareCompleted {
			s.Log("Cancelling receiving")
			if err := screenshot.CaptureChrome(ctx, cr, filepath.Join(s.OutDir(), "after_sharing.png")); err != nil {
				s.Log("Failed to capture a screenshot before cancelling receiving")
			}
			if err := androidDevice.CancelReceivingFile(ctx); err != nil {
				s.Error("Failed to cancel receiving after the share failed: ", err)
			}
			if err := androidDevice.AwaitSharingStopped(ctx, testTimeout); err != nil {
				s.Error("Failed waiting for the Android device to signal that sharing has finished: ", err)
			}
		}
	}()

	s.Log("Waiting for CrOS sender to detect Android receiver")
	if err := sender.SelectShareTarget(ctx, androidDisplayName, nearbyshare.DetectShareTargetTimeout); err != nil {
		s.Fatal("CrOS device failed to select Android device as a receiver and start the transfer: ", err)
	}

	s.Log("Waiting for Android receiver to detect the incoming share from CrOS sender")
	if err := androidDevice.AwaitReceiverConfirmation(ctx, nearbyshare.DetectShareTargetTimeout); err != nil {
		s.Fatal("Failed waiting for the Android device to detect the share: ", err)
	}

	// Get the secure sharing token to confirm the share on Android.
	token, err := sender.ConfirmationToken(ctx)
	if err != nil {
		s.Fatal("Failed to get confirmation token: ", err)
	}

	s.Log("Accepting the share on the Android receiver")
	if err := androidDevice.AcceptTheSharing(ctx, token); err != nil {
		s.Fatal("Failed to accept the share on the Android device: ", err)
	}

	s.Log("Waiting for the Android receiver to signal that sharing has completed")
	if err := androidDevice.AwaitSharingStopped(ctx, testData.TransferTimeout); err != nil {
		s.Fatal("Failed waiting for the Android device to signal that sharing has finished: ", err)
	}
	shareCompleted = true

	// Hash the file on both sides and confirm they match. Android receives shares in its default downloads directory.
	if err := nearbytestutils.FileHashComparison(ctx, filenames, nearbytestutils.SendDir, android.DownloadDir, androidDevice); err != nil {
		s.Fatal("Failed file hash comparison: ", err)
	}
	s.Log("Share completed and file hashes match on both sides")
}
