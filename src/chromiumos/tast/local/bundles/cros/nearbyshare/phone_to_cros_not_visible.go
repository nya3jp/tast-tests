// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PhoneToCrosNotVisible,
		Desc: "Checks that CrOS device won't be found based on its visibility setting",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		// TODO(crbug/1127165) Move to fixture when data is available.
		Data: []string{nearbysnippet.ZipName, nearbysnippet.AccountUtilZip},
		Params: []testing.Param{
			{
				Name:    "somecontacts",
				Fixture: "nearbyShareDataUsageOfflineSomeContactsAndroidNotSelectedContactGAIA",
				Val: nearbytestutils.TestData{
					Filename:    "small_jpg.zip",
					TestTimeout: nearbyshare.DetectionTimeout,
					MimeType:    nearbysnippet.MimeTypeJpeg,
				},
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbyshare.DetectionTimeout,
			},
			{
				Name:    "noone",
				Fixture: "nearbyShareDataUsageOnlineNoOneGAIA",
				Val: nearbytestutils.TestData{
					Filename:    "small_jpg.zip",
					TestTimeout: nearbyshare.DetectionTimeout,
					MimeType:    nearbysnippet.MimeTypeJpeg,
				},
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbyshare.DetectionTimeout,
			},
		},
	})
}

// PhoneToCrosNotVisible tests in-contact file sharing with an Android device as sender and CrOS device as receiver.
func PhoneToCrosNotVisible(ctx context.Context, s *testing.State) {
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
	defer androidDevice.AwaitSharingStopped(ctx, 10*time.Second)
	defer androidDevice.CancelSendingFile(ctx)

	s.Logf("Waiting for %v seconds to ensure no incoming share notification is displayed", nearbyshare.DetectShareTargetTimeout.Seconds())
	testing.Sleep(ctx, nearbyshare.DetectShareTargetTimeout)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	if exists, err := nearbyshare.IncomingShareNotificationExists(ctx, tconn, androidDisplayName); err != nil {
		s.Fatal("Failed to check if CrOS incoming share notification exists: ", err)
	} else if exists {
		s.Fatal("Incoming share notification found unexpectedly; CrOS device should not have been visible to the sender")
	}
}
