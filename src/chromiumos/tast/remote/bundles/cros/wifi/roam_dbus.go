// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"io"
	"time"

	"chromiumos/tast/common/network/protoutil"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	ap1BSSID   = "00:11:22:33:44:55"
	ap1Channel = 48
	ap2BSSID   = "00:11:22:33:44:56"
	ap2Channel = 1
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RoamDbus,
		Desc:     "Tests an intentional client-driven roam between APs",
		Contacts: []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:     []string{"group:wificell", "wificell_func", "wificell_unstable"},
		// These boards have Broadcom WiFi chip which doesn't support supplicant-based roaming.
		HardwareDeps: hwdep.D(hwdep.SkipOnPlatform("veyron_mickey"), hwdep.SkipOnPlatform("veyron_speedy")),
		ServiceDeps:  []string{wificell.TFServiceName},
		Pre:          wificell.TestFixturePre(),
		Vars:         []string{"router", "pcap"},
	})
}

func RoamDbus(ctx context.Context, s *testing.State) {
	// This test seeks to associate the DUT with an AP with a set of
	// association parameters, creates a second AP with a different set of
	// association parameters but the same SSID, and sends roam command to
	// shill. After that shill will send a D-Bus roam command to wpa_supplicant.
	// We seek to observe that the DUT successfully connects to the second
	// AP in a reasonable amount of time.
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	// Configure the initial AP.
	optionsAP1 := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(ap1Channel), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.BSSID(ap1BSSID)}
	ap1, err := tf.ConfigureAP(ctx, optionsAP1, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap1); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(ctx)

	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()

	ap1SSID := ap1.Config().SSID

	// Connect to the initial AP.
	if _, err := tf.ConnectWifiAP(ctx, ap1); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	}

	defer func(ctx context.Context) {
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap1SSID)}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap1BSSID, err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := tf.VerifyConnection(ctx, ap1); err != nil {
		s.Fatal("DUT: failed to verify connection: ", err)
	}

	// Prepare the state list for ExpectIn.
	var connectedStates []*network.ShillVal
	for _, shillState := range shillconst.ServiceConnectedStates {
		state, err := protoutil.ToShillVal(shillState)
		if err != nil {
			s.Fatal("Failed to convert property name to ShillVal, err: ", err)
		}
		connectedStates = append(connectedStates, state)
	}

	config, err := protoutil.ToShillVal(shillconst.ServiceStateConfiguration)
	if err != nil {
		s.Fatal("Failed to convert property name to ShillVal, err: ", err)
	}

	idle, err := protoutil.ToShillVal(shillconst.ServiceStateIdle)
	if err != nil {
		s.Fatal("Failed to convert property name to ShillVal, err: ", err)
	}

	roamToBSSID, err := protoutil.ToShillVal(ap2BSSID)
	if err != nil {
		s.Fatal("Failed to convert property name to ShillVal, err: ", err)
	}

	req := &network.ExpectShillPropertyValuesRequest{
		ServicePath: tf.GetServicePath(ctx),
		Props: []*network.ExpectShillPropertyValuesRequest_Entry{
			&network.ExpectShillPropertyValuesRequest_Entry{
				Key:    shillconst.ServicePropertyState,
				AnyOf:  []*network.ShillVal{config},
				NoneOf: []*network.ShillVal{idle},
				Check:  false},

			&network.ExpectShillPropertyValuesRequest_Entry{
				Key:    shillconst.ServicePropertyState,
				AnyOf:  connectedStates,
				NoneOf: []*network.ShillVal{idle},
				Check:  false},

			&network.ExpectShillPropertyValuesRequest_Entry{
				Key:    shillconst.ServicePropertyWiFiBSSID,
				AnyOf:  []*network.ShillVal{roamToBSSID},
				NoneOf: []*network.ShillVal{},
				Check:  true}},
	}

	stream, err := tf.WifiClient().ExpectShillPropertyValues(ctx, req)
	ready, err := stream.Recv()
	if ready.Key != "" {
		// Error due to expecting an empty response as ready signal.
		s.Fatal("Failed to get the ready signal")
	}

	// Setup the second AP interface on the same device with the same
	// SSID, but on different band (5 GHz for AP1 and 2.4 GHz for AP2).
	optionsAP2 := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(ap2Channel), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.SSID(ap1SSID), hostapd.BSSID(ap2BSSID)}
	ap2, err := tf.ConfigureAP(ctx, optionsAP2, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap2); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(ctx)

	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap2)
	defer cancel()

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("DUT: ", err)
	}

	if err := tf.DiscoverBSSID(ctx, ap2BSSID, iface); err != nil {
		s.Fatalf("DUT: failed to find the BSSID %s: %v", ap2BSSID, err)
	}

	// Send roam command to shill, and shill will send dbus roam command to wpa_supplicant.
	s.Logf("Requesting roam from %s to %s", ap1BSSID, ap2BSSID)
	if err := tf.RequestRoam(ctx, iface, ap2BSSID, 30*time.Second); err != nil {
		s.Errorf("DUT: failed to roam from %s to %s: %v", ap1BSSID, ap2BSSID, err)
	}

	// Get the expected properties and values.
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.Fatal("Failed to get the expected properties, err: ", err)
		}
		// Now we get the matched state change in resp.
		stateVal, err := protoutil.FromShillVal(resp.Val)
		if err != nil {
			s.Fatal("Failed to convert property name to ShillVal, err: ", err)
		}
		s.Logf("DUT: the current WiFi service %s = %s", resp.Key, stateVal)
	}

	s.Log("DUT: roamed")

	if err := tf.VerifyConnection(ctx, ap2); err != nil {
		s.Fatal("DUT: failed to verify connection: ", err)
	}

}
