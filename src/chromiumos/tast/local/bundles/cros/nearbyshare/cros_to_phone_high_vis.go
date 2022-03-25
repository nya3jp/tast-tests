// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"path/filepath"

	"chromiumos/tast/common/android"
	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbyfixture"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrosToPhoneHighVis,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that we can successfully send files from a CrOS to Android",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:    "dataoffline_noone_png5kb",
				Fixture: "nearbyShareDataUsageOfflineNoOne",
				Val: nearbycommon.TestData{
					Filename:        "small_png.zip",
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
				},
				ExtraData: []string{"small_png.zip"},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
			},
			{
				Name:    "dataoffline_noone_jpg11kb",
				Fixture: "nearbyShareDataUsageOfflineNoOne",
				Val: nearbycommon.TestData{
					Filename:        "small_jpg.zip",
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
				},
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
			},
			{
				Name:    "dataonline_noone_txt30mb",
				Fixture: "nearbyShareDataUsageOnlineNoOne",
				Val: nearbycommon.TestData{
					Filename:        "big_txt.zip",
					TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
				},
				ExtraData: []string{"big_txt.zip"},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
			},
			{
				Name:    "dataonline_noone_txt30mb_webrtc_and_wlan",
				Fixture: "nearbyShareDataUsageOnlineNoOneWebRTCAndWLAN",
				Val: nearbycommon.TestData{
					Filename:        "big_txt.zip",
					TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
				},
				ExtraData: []string{"big_txt.zip"},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
			},
			{
				Name:    "dataonline_noone_txt30mb_webrtc",
				Fixture: "nearbyShareDataUsageOnlineNoOneWebRTCOnly",
				Val: nearbycommon.TestData{
					Filename:        "big_txt.zip",
					TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
				},
				ExtraData: []string{"big_txt.zip"},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
			},
			{
				Name:    "dataonline_noone_txt30mb_wlan",
				Fixture: "nearbyShareDataUsageOnlineNoOneWLANOnly",
				Val: nearbycommon.TestData{
					Filename:        "big_txt.zip",
					TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
				},
				ExtraData: []string{"big_txt.zip"},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
			},
		},
	})
}

// CrosToPhoneHighVis tests file sharing with a CrOS device as sender and Android device as receiver.
func CrosToPhoneHighVis(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*nearbyfixture.FixtData).Chrome
	tconn := s.FixtValue().(*nearbyfixture.FixtData).TestConn
	crosDisplayName := s.FixtValue().(*nearbyfixture.FixtData).CrOSDeviceName
	androidDevice := s.FixtValue().(*nearbyfixture.FixtData).AndroidDevice
	androidDisplayName := s.FixtValue().(*nearbyfixture.FixtData).AndroidDeviceName

	// Extract the test file(s) to nearbyshare.SendDir.
	testData := s.Param().(nearbycommon.TestData)
	testDataZip := s.DataPath(testData.Filename)
	filenames, err := nearbytestutils.ExtractCrosTestFiles(ctx, testDataZip)
	if err != nil {
		s.Fatal("Failed to extract test data files: ", err)
	}

	// Get the full paths of the test files to pass to chrome://nearby.
	var testFiles []string
	for _, f := range filenames {
		testFiles = append(testFiles, filepath.Join(nearbycommon.SendDir, f))
	}

	s.Log("Starting sending on the CrOS device")
	sender, err := nearbyshare.StartSendFiles(ctx, cr, testFiles)
	if err != nil {
		s.Fatal("Failed to set up control over the send surface: ", err)
	}
	defer sender.Close(ctx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Starting high-visibility receiving on the Android device")
	testTimeout := testData.TestTimeout
	if err := androidDevice.ReceiveFile(ctx, crosDisplayName, androidDisplayName, true, testTimeout); err != nil {
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
	if err := sender.SelectShareTarget(ctx, androidDisplayName, nearbycommon.DetectShareTargetTimeout); err != nil {
		s.Fatal("CrOS device failed to select Android device as a receiver and start the transfer: ", err)
	}

	s.Log("Waiting for Android receiver to detect the incoming share from CrOS sender")
	if err := androidDevice.AwaitReceiverConfirmation(ctx, nearbycommon.DetectShareTargetTimeout); err != nil {
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
	if err := nearbytestutils.FileHashComparison(ctx, filenames, nearbycommon.SendDir, android.DownloadDir, androidDevice); err != nil {
		s.Fatal("Failed file hash comparison: ", err)
	}
	s.Log("Share completed and file hashes match on both sides")
}
