// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/local/shill"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        RoamDbus,
		Desc:        "Tests an intentional client-driven roam between APs",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

func RoamDbus(fullCtx context.Context, s *testing.State) {
	// This test seeks to associate the DUT with an AP with a set of
	// association parameters, create a second AP with a second set of
	// parameters but the same SSID, and send roam command to shill. After
	// that shill will send a dbus roam command to wpa_supplicant. We seek
	// to observe that the DUT successfully connects to the second AP in
	// a reasonable amount of time.
	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(fullCtx, fullCtx, s.DUT(), s.RPCHint(), wificell.TFRouter(router))
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(fullCtx); err != nil {
			s.Log("Failed to tear down test fixture: ", err)
		}
	}()

	tfCtx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	// Configure the inital AP.
	optionsAP1 := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT20)}
	ap1, err := tf.ConfigureAP(tfCtx, optionsAP1, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(tfCtx, ap1); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}()

	apCtx, cancel := tf.ReserveForDeconfigAP(tfCtx, ap1)
	defer cancel()

	// Connect to the initial AP.
	if _, err := tf.ConnectWifiAP(apCtx, ap1); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	}

	defer func() {
		if err := tf.DisconnectWifi(apCtx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(apCtx, &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap1.Config().SSID)}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap1.Config().SSID, err)
		}
	}()

	// Setup a second AP with the same SSID.
	optionsAP2 := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(1), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.SSID(ap1.Config().SSID)}
	ap2, err := tf.ConfigureAP(apCtx, optionsAP2, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(apCtx, ap2); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}()

	ctx, cancel := tf.ReserveForDeconfigAP(apCtx, ap2)
	defer cancel()

	// Get BSSIDs of the two APs.
	bssid1 := ap1.Config().BSSID
	s.Logf("AP1 BSSID = %s", bssid1)
	bssid2 := ap2.Config().BSSID
	s.Logf("AP2 BSSID = %s", bssid2)

	// Wait for DUT to see the second AP.
	/*
		var scanData *iw.TimedScanData
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			scanData, err = remoteiw.NewRemoteRunner(s.DUT().Conn()).TimedScan(ctx, iface, bssid2)
			if err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond}); err != nil {
			s.Fatal("DUT: failed to run TimedScan: ", err)
		}
	*/
	props := map[string]interface{}{
		shill.ServicePropertyType:      shill.TypeWifi,
		shill.ServicePropertyWiFiBSSID: bssid1,
	}

	if _, err := tf.DiscoverService(ctx, props); err != nil {
		s.Fatalf("DUT: failed to find the BSSID %s: %v", bssid2, err)
	}

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("DUT: ", err)
	}

	// Check which AP we are currently connected.
	// This is to include the case that wpa_supplicant
	// automatically roam to AP2 during the scan.
	currBSSID, err := remoteiw.NewRemoteRunner(s.DUT().Conn()).CurrentBSSID(ctx, iface)
	if err != nil {
		s.Fatal("DUT: failed to get the current BSSID: ", err)
	}
	roamToBSSID := bssid2
	if currBSSID == bssid2 {
		roamToBSSID = bssid1
	}

	s.Logf("Requesting roam from %s to %s", currBSSID, roamToBSSID)
	/*
		// Send roam command to shill, and shill will send dbus roam command to wpa_supplicant.
		if not self.context.client.request_roam_dbus(roam_to_bssid, interface):
			s.Fatal("DUT: failed to send roam command: ", err)

		// Expect that the DUT will re-connect to the new AP.
		if not self.context.client.wait_for_roam(roam_to_bssid, timeout_seconds=self.TIMEOUT_SECONDS):
			s.Fatal("DUT: failed to roam: ", err)
	*/
}
