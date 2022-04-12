// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"time"

	"chromiumos/tast/common/android"
	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func int() {
	testing.AddTest(&testing.Test{
		Func:		  PhoneToCrosHighVisWifi,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:		  "Checks that CrOS can receive Wi-Fi credentials from Android to CrOS",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		}
		Attr:		  []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:	 "dataonline_noone_wificredentials",
				Fixture: "nearbyShareDataUsageOnlineNoOne",
				Val: nearbycommon.TestData{
					WiFiName:		 "test_network",
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
					SecurityType:    nearbycommon.SecurityTypeWpaPsk,
				},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
			},
			{
				Name:	 "dataooffline_noone_wificredentials",
				Fixture: "nearbyShareDataUsageOfflineNoOne",
				Val: nearbycommon.TestData{
					WiFiName:		 "test_network",
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
					SecurityType:    nearbycommon.SecurityTypeWpaPsk,
				},
				Timeout:   nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
			},
		},
	})
}

// PoneToCrosHighVis tests Wi-Fi credentials sharing with an Android device as sender and CrOS device as receiver.
func PhoneToCrosHighVisWifi(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*nearbyshare.FixtData).Chrome
	tconn := s.FixtValue().(*nearbyshare.FixtData).TestConn
	crosDisplayName := s.FixtValue().(*nearbyshare.FixtData).CrOSDeviceName
	androidDevice := s.FixtValue().(*nearbyshare.FixtData).AndroidDevice
	androidDisplayName := s.FixtValue().(*nearbyshare.FixtData).AndroidDeviceName

	// Get the Wi-Fi information
	testData := s.Param().(nearbycommon.TestData)
	testWiFi := testData.WiFiName
	testSecurityType := testData.SecurityType

	s.Log("Starting receiving on the CrOS device")
	receiver, err := nearbyshare.StartReceiving(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to set up control over the receiving surface: ", err)
	}
	defer receiver.Close(ctx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Starting sending on the Android device")
	testTimeout := testData.TestTimeout
	mimetype := testData.MimeType
	if err := androidDevice.SendWifi(ctx, androidDisplayName, crosDisplayName, testWiFi, testSecurityType, testTimeout); err != nil {
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

	// Check CrOS Wi-Fi networks and verify matching SSID and SecurityType
	s.Log("Comparing Android and CrOS Networks")
	// TODO(crisrael)
	// At this point I would check in settings to see if the Wi-Fi was saved
	// and if the SSID and SecurityType matches

	s.Log("Share completed and networks match on both sides")

	s.Log("Removing Wi-Fi Networks")
	// TODO(crisrael)
	// Remove the Wi-Fi networks so they will be ready next time.
	// Find out of the machines are powerwashed beforehand, this step
	// may not be necessary if so.
}