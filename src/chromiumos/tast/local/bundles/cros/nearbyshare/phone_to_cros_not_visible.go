// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"strings"
	"time"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbyfixture"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhoneToCrosNotVisible,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that CrOS device won't be found based on its visibility setting",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"kyleshima@chromium.org",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:    "somecontacts",
				Fixture: "nearbyShareDataUsageOfflineSomeContactsAndroidNotSelectedContact",
				Val: nearbycommon.TestData{
					Filename:    "small_jpg.zip",
					TestTimeout: nearbycommon.DetectionTimeout,
					MimeType:    nearbycommon.MimeTypeJpeg,
				},
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbycommon.DetectionTimeout,
			},
			{
				Name:    "noone",
				Fixture: "nearbyShareDataUsageOnlineNoOne",
				Val: nearbycommon.TestData{
					Filename:    "small_jpg.zip",
					TestTimeout: nearbycommon.DetectionTimeout,
					MimeType:    nearbycommon.MimeTypeJpeg,
				},
				ExtraData: []string{"small_jpg.zip"},
				Timeout:   nearbycommon.DetectionTimeout,
			},
		},
	})
}

// PhoneToCrosNotVisible tests in-contact file sharing with an Android device as sender and CrOS device as receiver.
func PhoneToCrosNotVisible(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*nearbyfixture.FixtData).TestConn
	crosDisplayName := s.FixtValue().(*nearbyfixture.FixtData).CrOSDeviceName
	androidDevice := s.FixtValue().(*nearbyfixture.FixtData).AndroidDevice
	androidDisplayName := s.FixtValue().(*nearbyfixture.FixtData).AndroidDeviceName

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
	defer androidDevice.AwaitSharingStopped(ctx, 10*time.Second)
	defer androidDevice.CancelSendingFile(ctx)

	s.Logf("Waiting for %v seconds to ensure no incoming share notification is displayed", nearbycommon.DetectShareTargetTimeout.Seconds())
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	err = testing.Poll(ctx, func(ctx context.Context) error {
		if exists, err := nearbyshare.IncomingShareNotificationExists(ctx, tconn, androidDisplayName); err != nil {
			return testing.PollBreak(err)
		} else if exists {
			return testing.PollBreak(errors.New("incoming share notification found unexpectedly"))
		}
		return errors.New("continuing to wait until time elapsed")
	}, &testing.PollOptions{Timeout: nearbycommon.DetectShareTargetTimeout, Interval: time.Second})
	if !strings.Contains(err.Error(), "continuing to wait until time elapsed") {
		s.Fatal("Notification check failed: ", err)
	}
}
