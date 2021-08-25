// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"path/filepath"
	"time"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/common/cros/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/android"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PhoneToCrosInContacts,
		Desc: "Checks that we can successfully send files between contacts from Android to CrOS",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:    "dataoffline_allcontacts_jpg11kb",
				Fixture: "nearbyShareDataUsageOfflineAllContacts",
				Val: nearbytestutils.TestData{
					Filename:        "small_jpg.zip",
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
					MimeType:        nearbysnippet.MimeTypeJpeg,
				},
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
			},
			{
				Name:    "dataoffline_allcontacts_png5kb",
				Fixture: "nearbyShareDataUsageOfflineAllContacts",
				Val: nearbytestutils.TestData{
					Filename:        "small_png.zip",
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
					MimeType:        nearbysnippet.MimeTypePng,
				},
				ExtraData: []string{"small_png.zip"},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
			},
			{
				Name:    "dataoffline_somecontacts_jpg11kb",
				Fixture: "nearbyShareDataUsageOfflineSomeContactsAndroidSelectedContact",
				Val: nearbytestutils.TestData{
					Filename:        "small_jpg.zip",
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
					MimeType:        nearbysnippet.MimeTypeJpeg,
				},
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
			},
			{
				Name:    "dataoffline_somecontacts_png5kb",
				Fixture: "nearbyShareDataUsageOfflineSomeContactsAndroidSelectedContact",
				Val: nearbytestutils.TestData{
					Filename:        "small_png.zip",
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
					MimeType:        nearbysnippet.MimeTypePng,
				},
				ExtraData: []string{"small_png.zip"},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
			},
			{
				Name:    "dataonline_allcontacts_txt30mb",
				Fixture: "nearbyShareDataUsageOnlineAllContacts",
				Val: nearbytestutils.TestData{
					Filename:        "big_txt.zip",
					TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
					MimeType:        nearbysnippet.MimeTypeTextPlain,
				},
				ExtraData: []string{"big_txt.zip"},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
			},
			{
				Name:    "dataonline_somecontacts_txt30mb",
				Fixture: "nearbyShareDataUsageOnlineSomeContactsAndroidSelectedContact",
				Val: nearbytestutils.TestData{
					Filename:        "big_txt.zip",
					TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
					MimeType:        nearbysnippet.MimeTypeTextPlain,
				},
				ExtraData: []string{"big_txt.zip"},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
			},
		},
	})
}

// PhoneToCrosInContacts tests in-contact file sharing with an Android device as sender and CrOS device as receiver.
func PhoneToCrosInContacts(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*nearbyshare.FixtData).Chrome
	tconn := s.FixtValue().(*nearbyshare.FixtData).TestConn
	crosDisplayName := s.FixtValue().(*nearbyshare.FixtData).CrOSDeviceName
	androidDevice := s.FixtValue().(*nearbyshare.FixtData).AndroidDevice
	androidDisplayName := s.FixtValue().(*nearbyshare.FixtData).AndroidDeviceName

	// Extract the test file to the staging directory on the Android device.
	testData := s.Param().(nearbytestutils.TestData)
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

	s.Log("Waiting for incoming share notification on CrOS receiver")
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	if err := nearbyshare.AcceptIncomingShareNotification(ctx, tconn, androidDisplayName, nearbycommon.DetectShareTargetTimeout); err != nil {
		s.Fatal("CrOS receiver failed to find Android sender: ", err)
	}
	s.Log("Accepted the share on the CrOS receiver")

	s.Log("Waiting for the Android sender to signal that sharing has completed")
	if err := androidDevice.AwaitSharingStopped(ctx, testData.TransferTimeout); err != nil {
		s.Fatal("Failed waiting for the Android device to signal that sharing has finished: ", err)
	}
	shareCompleted = true

	s.Log("Waiting for receiving-complete notification on CrOS receiver")
	if err := nearbyshare.WaitForReceivingCompleteNotification(ctx, tconn, androidDisplayName, 10*time.Second); err != nil {
		s.Fatal("Failed waiting for notification to indicate sharing has completed on CrOS: ", err)
	}

	s.Log("Comparing Android and CrOS file hashes")
	if err := nearbytestutils.FileHashComparison(ctx, []string{testFile}, filesapp.DownloadPath, filepath.Join(android.DownloadDir, nearbysnippet.SendDir), androidDevice); err != nil {
		s.Fatal("Failed file hash comparison: ", err)
	}
	s.Log("Share completed and file hashes match on both sides")
}
