// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"encoding/hex"
	"strings"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbyfixture"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhoneToCrosHighVisWifiCredentials,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that CrOS can receive Wi-Fi credentials from Android to CrOS",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"crisrael@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:    "dataonline_noone_wificredentials",
				Fixture: "nearbyShareDataUsageOnlineNoOne",
				Val: nearbycommon.WiFiTestData{
					WiFiName:        "test_network",
					WiFiPassword:    "testpassword0000",
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
					SecurityType:    nearbycommon.SecurityTypeWpaPsk,
				},
				Timeout: nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
			},
		},
	})
}

// PhoneToCrosHighVisWifiCredentials tests Wi-Fi credentials sharing with an Android device as sender and CrOS device as receiver.
func PhoneToCrosHighVisWifiCredentials(ctx context.Context, s *testing.State) {
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

	// Check CrOS Wi-Fi networks to see if the network was saved
	s.Log("Searching for received Wi-Fi network")
	props := map[string]interface{}{
		shillconst.ServicePropertyType:        shillconst.TypeWifi,
		shillconst.ServicePropertyWiFiHexSSID: strings.ToUpper(hex.EncodeToString([]byte(testWiFi))),
		shillconst.ServicePropertyVisible:     true,
	}

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create a shill manager: ", err)
	}

	service, err := manager.FindMatchingService(ctx, props)
	if err != nil {
		s.Fatal("Failed to find the Wi-Fi network: ", err)
	}

	s.Log("Wi-Fi network found, now removing Wi-Fi network for cleanup")
	if err := service.Remove(ctx); err != nil {
		s.Fatal("Failed to remove the service")
	}
}
