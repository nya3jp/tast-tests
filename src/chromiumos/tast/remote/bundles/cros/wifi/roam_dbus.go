// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

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
	// association parameters, creates a second AP with a different set of
	// association parameters but the same SSID, and sends roam command to
	// shill. After that shill will send a D-Bus roam command to wpa_supplicant.
	// We seek to observe that the DUT successfully connects to the second
	// AP in a reasonable amount of time.
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

	// Configure the initial AP.
	const ap1BSSID = "00:11:22:33:44:55"
	const ap1Channel = 48
	optionsAP1 := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(ap1Channel), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.BSSID(ap1BSSID)}
	ap1, err := tf.ConfigureAP(tfCtx, optionsAP1, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(tfCtx, ap1); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}()
	ap1SSID := ap1.Config().SSID

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
		if _, err := tf.WifiClient().DeleteEntriesForSSID(apCtx, &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap1SSID)}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap1BSSID, err)
		}
	}()

	// Setup the second AP interface on the same device with the same
	// SSID, but on different band (5 GHz for AP1 and 2.4 GHz for AP2).
	const ap2BSSID = "00:11:22:33:44:56"
	const ap2Channel = 1
	optionsAP2 := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(ap2Channel), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.SSID(ap1SSID), hostapd.BSSID(ap2BSSID)}
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

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("DUT: ", err)
	}

	if err := tf.DiscoverBSSID(ctx, ap2BSSID, iface); err != nil {
		s.Fatalf("DUT: failed to find the BSSID %s: %v", ap2BSSID, err)
	}

	// Check which AP we are currently connected to. This is to include the case
	// that wpa_supplicant automatically roamed to AP2 during the scan.
	currBSSID, err := remoteiw.NewRemoteRunner(s.DUT().Conn()).CurrentBSSID(ctx, iface)
	if err != nil {
		s.Fatal("DUT: failed to get the current BSSID: ", err)
	}
	roamToBSSID := ap2BSSID
	if currBSSID == ap2BSSID {
		roamToBSSID = ap1BSSID
	}

	// Send roam command to shill, and shill will send dbus roam command to wpa_supplicant.
	s.Logf("Requesting roam from %s to %s", currBSSID, roamToBSSID)
	if err := tf.RequestRoam(ctx, iface, roamToBSSID, 10*time.Second); err != nil {
		s.Fatalf("DUT: failed to roam from current BSSID = %s to the BSSID = %s: %v", currBSSID, roamToBSSID, err)
	}

}
