// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RoamDbus,
		Desc: "Tests an intentional client-driven roam between APs",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func RoamDbus(ctx context.Context, s *testing.State) {
	// This test seeks to associate the DUT with an AP with a set of
	// association parameters. Then it creates a second AP with a different
	// set of association parameters but the same SSID, and sends roam
	// command to shill. After receiving the roam command, shill sends a D-Bus
	// roam command to wpa_supplicant. The test expects that the DUT
	// successfully connects to the second AP within a reasonable amount of time.
	tf := s.FixtValue().(*wificell.TestFixture)

	const (
		ap1Channel = 48
		ap2Channel = 1
	)
	// Generate BSSIDs for the two APs.
	mac1, err := hostapd.RandomMAC()
	if err != nil {
		s.Fatal("Failed to generate BSSID: ", err)
	}
	mac2, err := hostapd.RandomMAC()
	if err != nil {
		s.Fatal("Failed to generate BSSID: ", err)
	}
	ap1BSSID := mac1.String()
	ap2BSSID := mac2.String()

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
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()

	ap1SSID := ap1.Config().SSID

	// Connect to the initial AP.
	var servicePath string
	if resp, err := tf.ConnectWifiAP(ctx, ap1); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	} else {
		servicePath = resp.ServicePath
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.VerifyConnection(ctx, ap1); err != nil {
		s.Fatal("DUT: failed to verify connection: ", err)
	}

	props := []*wificell.ShillProperty{{
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateConfiguration},
		Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateReady},
		Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateIdle},
		Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiBSSID,
		ExpectedValues: []interface{}{ap2BSSID},
		Method:         wifi.ExpectShillPropertyRequest_CHECK_ONLY,
	}}

	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	monitorProps := []string{shillconst.ServicePropertyIsConnected}
	waitForProps, err := tf.ExpectShillProperty(waitCtx, servicePath, props, monitorProps)
	if err != nil {
		s.Fatal("DUT: failed to create a property watcher, err: ", err)
	}

	// Set up the second AP interface on the same device with the same
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

	discCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := tf.DiscoverBSSID(discCtx, ap2BSSID, iface, []byte(ap1SSID)); err != nil {
		s.Fatalf("DUT: failed to find the BSSID %s: %v", ap2BSSID, err)
	}

	// Send roam command to shill, and shill will send D-Bus roam command to wpa_supplicant.
	s.Logf("Requesting roam from %s to %s", ap1BSSID, ap2BSSID)
	if err := tf.RequestRoam(ctx, iface, ap2BSSID, 30*time.Second); err != nil {
		s.Errorf("DUT: failed to roam from %s to %s: %v", ap1BSSID, ap2BSSID, err)
	}

	monitorResult, err := waitForProps()
	if err != nil {
		s.Fatal("DUT: failed to wait for the properties, err: ", err)
	}
	s.Log("DUT: roamed")

	// Assert there was no disconnection during roaming.
	for _, ph := range monitorResult {
		if ph.Name == shillconst.ServicePropertyIsConnected {
			if !ph.Value.(bool) {
				s.Fatal("DUT: failed to stay connected during the roaming process")
			}
		}
	}

	if err := tf.VerifyConnection(ctx, ap2); err != nil {
		s.Fatal("DUT: failed to verify connection: ", err)
	}
}
