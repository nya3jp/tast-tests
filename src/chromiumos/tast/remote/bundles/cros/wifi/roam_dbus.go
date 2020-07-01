// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

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

	channels := []int{48, 1}
	ssid := hostapd.RandomSSID("TAST_TEST_")

	// Configure the initial AP.
	const ap1BSSID = "00:11:22:33:44:55"
	optionsAP1 := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(channels[0]), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.BSSID(ap1BSSID)}
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
		if _, err := tf.WifiClient().DeleteEntriesForSSID(apCtx, &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ssid)}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ssid, err)
		}
	}()

	// Setup the second AP interface on the same device with the same
	// SSID, but on different band (5 GHz for AP1 and 2.4 GHz for AP2).
	const ap2BSSID = "00:11:22:33:44:56"
	optionsAP2 := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(channels[1]), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.SSID(ssid), hostapd.BSSID(ap2BSSID)}
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

	var freqs []int
	for _, ch := range channels {
		freq, err := hostapd.ChannelToFrequency(ch)
		if err != nil {
			s.Fatalf("Invalid channel %d: %v", ch, err)
		}
		freqs = append(freqs, freq)
	}

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("DUT: ", err)
	}

	props := map[string]interface{}{
		shill.ServicePropertyType:      shill.TypeWifi,
		shill.ServicePropertyWiFiBSSID: ap2BSSID,
	}

	if _, err := tf.DiscoverService(ctx, props); err != nil {
		s.Fatalf("DUT: failed to find the BSSID %s: %v", ap2BSSID, err)
	}

	// Check which AP we are currently connected.
	// This is to include the case that wpa_supplicant
	// automatically roam to AP2 during the scan.
	currBSSID, err := remoteiw.NewRemoteRunner(s.DUT().Conn()).CurrentBSSID(ctx, iface)
	if err != nil {
		s.Fatal("DUT: failed to get the current BSSID: ", err)
	}
	roamToBSSID := ap2BSSID
	if currBSSID == ap2BSSID {
		roamToBSSID = ap1BSSID
	}

	s.Logf("Requesting roam from %s to %s", currBSSID, roamToBSSID)
	// Send roam command to shill, and shill will send dbus roam command to wpa_supplicant.
	if err := tf.RequestRoam(ctx, iface, roamToBSSID, 10*time.Second); err != nil {
		s.Fatalf("DUT: failed to roam from currnt BSSID = %s to the BSSID = %s: %v", currBSSID, roamToBSSID, err)
	}

	// testing.Sleep(ctx, 60*time.Second)

	currBSSID, err = remoteiw.NewRemoteRunner(s.DUT().Conn()).CurrentBSSID(ctx, iface)
	if err != nil {
		s.Fatal("DUT: failed to get the current BSSID: ", err)
	}
	s.Logf("Current AP = %s", currBSSID)
}
