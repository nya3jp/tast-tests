// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbyfixture"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhoneToCrosWifiCredsUnsecureNetwork,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that ChromeOS rejects unsecure Wi-Fi credentials received from Android",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"crisrael@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:    "dataonline_noone_wificreds_unsecure",
				Fixture: "nearbyShareDataUsageOnlineNoOne",
				Val: nearbycommon.WiFiTestData{
					WiFiName:        "test_network",
					WiFiPassword:    "",
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
					SecurityType:    nearbycommon.SecurityTypeOpen,
				},
				Timeout: nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
			},
		},
	})
}

// PhoneToCrosWifiCredsUnsecureNetwork verifies that ChromeOS will reject Wi-Fi credentials received from Android if it's unsecure.
func PhoneToCrosWifiCredsUnsecureNetwork(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*nearbyfixture.FixtData).Chrome
	tconn := s.FixtValue().(*nearbyfixture.FixtData).TestConn
	crosDisplayName := s.FixtValue().(*nearbyfixture.FixtData).CrOSDeviceName
	androidDevice := s.FixtValue().(*nearbyfixture.FixtData).AndroidDevice
	androidDisplayName := s.FixtValue().(*nearbyfixture.FixtData).AndroidDeviceName

	// Get the Wi-Fi information
	testData := s.Param().(nearbycommon.WiFiTestData)
	testWiFi := testData.WiFiName
	testSecurityType := testData.SecurityType
	testPassword := testData.WiFiPassword

	s.Log("Starting receiving on the CrOS device")
	receiver, err := nearbyshare.StartReceiving(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to set up control over the receiving surface: ", err)
	}
	defer receiver.Close(ctx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Starting sending on the Android device")
	testTimeout := testData.TestTimeout
	if err := androidDevice.SendWifi(ctx, androidDisplayName, crosDisplayName, testWiFi, testPassword, testSecurityType, testTimeout); err != nil {
		s.Fatal("Failed to start sending on Android: ", err)
	}

	// Defer cancelling the share on the Android side if it does not succeed.
	var shareCompleted bool
	defer func() {
		if !shareCompleted {
			s.Log("Cancelling sending")
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

	s.Log("Verify we receive a notification that we could not save the network")
	if err := nearbyshare.VerifyCouldNotSaveWiFiNetwork(ctx, tconn, androidDisplayName, testWiFi, nearbycommon.WiFiNotificationTimeout); err != nil {
		s.Fatal("Failed to open known Wi-Fi networks from notification")
	}
}
