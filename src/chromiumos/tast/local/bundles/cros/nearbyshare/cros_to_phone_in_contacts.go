// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"path/filepath"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/common/cros/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/android"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosToPhoneInContacts,
		Desc: "Checks that we can successfully send files between contacts from CrOS to Android",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:    "dataoffline_allcontacts_png5kb",
				Fixture: "nearbyShareDataUsageOfflineAllContacts",
				Val: nearbytestutils.TestData{
					Filename:        "small_png.zip",
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
				},
				ExtraData: []string{"small_png.zip"},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
			},
			{
				Name:    "dataoffline_allcontacts_jpg11kb",
				Fixture: "nearbyShareDataUsageOfflineAllContacts",
				Val: nearbytestutils.TestData{
					Filename:        "small_jpg.zip",
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
				},
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
			},
			{
				Name:    "dataonline_allcontacts_txt30mb",
				Fixture: "nearbyShareDataUsageOnlineAllContacts",
				Val: nearbytestutils.TestData{
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

// CrosToPhoneInContacts tests in-contact file sharing with a CrOS device as sender and Android device as receiver.
func CrosToPhoneInContacts(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*nearbyshare.FixtData).Chrome
	tconn := s.FixtValue().(*nearbyshare.FixtData).TestConn
	crosDisplayName := s.FixtValue().(*nearbyshare.FixtData).CrOSDeviceName
	androidDevice := s.FixtValue().(*nearbyshare.FixtData).AndroidDevice
	androidDisplayName := s.FixtValue().(*nearbyshare.FixtData).AndroidDeviceName

	// Extract the test file(s) to nearbytestutils.SendDir.
	testData := s.Param().(nearbytestutils.TestData)
	testDataZip := s.DataPath(testData.Filename)
	filenames, err := nearbytestutils.ExtractCrosTestFiles(ctx, testDataZip)
	if err != nil {
		s.Fatal("Failed to extract test data files: ", err)
	}

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
	if err := sender.SelectShareTarget(ctx, androidDisplayName, nearbycommon.DetectShareTargetTimeout); err != nil {
		s.Fatal("CrOS device failed to select Android device as a receiver and start the transfer: ", err)
	}

	// TODO(b/179309645): Remove UI-based Android controls once API control is available.
	s.Log("Waiting for contacts receiving UI on Android receiver")
	if err := androidDevice.InitUI(ctx); err != nil {
		s.Fatal("Failed to start UI Automator: ", err)
	}
	defer androidDevice.CloseUI(ctx)
	if err := androidDevice.WaitForInContactSenderUI(ctx, crosDisplayName, nearbycommon.DetectShareTargetTimeout); err != nil {
		s.Fatal("Failed to find receive UI on the Android device: ", err)
	}

	s.Log("Accepting the share through the UI on the Android receiver")
	if err := androidDevice.AcceptUI(ctx, testData.TransferTimeout); err != nil {
		s.Fatal("Android failed to accept the share through the UI: ", err)
	}

	// Hash the file on both sides and confirm they match. Android receives shares in its default downloads directory.
	if err := nearbytestutils.FileHashComparison(ctx, filenames, nearbytestutils.SendDir, android.DownloadDir, androidDevice); err != nil {
		s.Fatal("Failed file hash comparison: ", err)
	}
	s.Log("Share completed and file hashes match on both sides")
}
