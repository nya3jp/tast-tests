// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"time"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbyfixture"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhoneToCrosBackgroundScanningDisabled,
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
				Name:    "no_notification_shown",
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

// PhoneToCrosBackgroundScanningDisabled tests that the background scanning notification does not appear if background scanning is toggled off with an Android device as sender and CrOS device as receiver.
func PhoneToCrosBackgroundScanningDisabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*nearbyfixture.FixtData).Chrome
	tconn := s.FixtValue().(*nearbyfixture.FixtData).TestConn
	crosDisplayName := s.FixtValue().(*nearbyfixture.FixtData).CrOSDeviceName
	androidDevice := s.FixtValue().(*nearbyfixture.FixtData).AndroidDevice
	androidDisplayName := s.FixtValue().(*nearbyfixture.FixtData).AndroidDeviceName

	if err := nearbyshare.ToggleNearbyDeviceIsSharingNotification(ctx, tconn, cr /*setChecked=*/, false); err != nil {
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
	defer androidDevice.AwaitSharingStopped(ctx, 10*time.Second)
	defer androidDevice.CancelSendingFile(ctx)

	s.Logf("Waiting for %v seconds to ensure no background scanning notification is displayed", nearbycommon.DetectShareTargetTimeout.Seconds())
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	start := time.Now()
	if err := testing.Poll(ctx,
		func(ctx context.Context) error {
			if exists, err := nearbyshare.FastInitiationNotificationExists(ctx, tconn); err != nil {
				return testing.PollBreak(err)
			} else if exists {
				return testing.PollBreak(errors.New("background scanning notification found unexpectedly"))
			}
			if time.Since(start) >= nearbycommon.DetectShareTargetTimeout {
				// Timeout is reached and element was not found.
				return nil
			}
			return errors.Errorf("still waiting for the node for %.1fs", (nearbycommon.DetectShareTargetTimeout - time.Since(start)).Seconds())
		},
		nil,
	); err != nil {
		s.Fatal("Failed to ensure that no background scanning notification is displayed: ", err)
	}
}
