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
		Func: BSSTMRequest,
		Desc: "Tests the DUTs response to a BSS Transition Management Request",
		Contacts: []string{
			"wgd@google.com",                  // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePreWithCapture(),
		Vars:        []string{"router", "pcap"},
	})
}

func BSSTMRequest(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	// Generate BSSIDs for the two APs.
	mac0, err := hostapd.RandomMAC()
	if err != nil {
		s.Fatal("Failed to generate BSSID: ", err)
	}
	mac1, err := hostapd.RandomMAC()
	if err != nil {
		s.Fatal("Failed to generate BSSID: ", err)
	}
	fromBSSID := mac0.String()
	roamBSSID := mac1.String()
	s.Log("AP 0 BSSID: ", fromBSSID)
	s.Log("AP 1 BSSID: ", roamBSSID)

	const (
		testSSID      = "BSS_TM"
		roamTimeout   = 30 * time.Second
		verifyTimeout = 20 * time.Second
	)
	apOpts0 := []hostapd.Option{hostapd.SSID(testSSID), hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(1), hostapd.BSSID(fromBSSID)}
	apOpts1 := []hostapd.Option{hostapd.SSID(testSSID), hostapd.Mode(hostapd.Mode80211a), hostapd.Channel(48), hostapd.BSSID(roamBSSID)}

	runTest := func(ctx context.Context, s *testing.State, waitForScan bool) {
		// Configure the first AP.
		s.Log("Configuring AP 0")
		ap0, err := tf.ConfigureAP(ctx, apOpts0, nil)
		if err != nil {
			s.Fatal("Failed to configure AP 0: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap0); err != nil {
				s.Error("Failed to deconfig AP 0: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap0)
		defer cancel()

		// Connect to the first AP.
		s.Log("Connecting to AP 0")
		cleanupCtx := ctx
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()
		connectResp, err := tf.ConnectWifiAP(ctx, ap0)
		if err != nil {
			s.Fatal("Failed to connect to AP 0: ", err)
		}
		servicePath := connectResp.ServicePath
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi: ", err)
			}
		}(cleanupCtx)
		s.Log("Verifying connection to AP 0")
		if err := tf.VerifyConnection(ctx, ap0); err != nil {
			s.Fatal("Failed to verify connection: ", err)
		}

		// Set up a watcher for the Shill WiFi BSSID property.
		monitorProps := []string{shillconst.ServicePropertyIsConnected}
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
			ExpectedValues: []interface{}{roamBSSID},
			Method:         wifi.ExpectShillPropertyRequest_CHECK_ONLY,
		}}

		waitCtx, cancel := context.WithTimeout(ctx, roamTimeout)
		defer cancel()
		waitForProps, err := tf.ExpectShillProperty(waitCtx, servicePath, props, monitorProps)
		if err != nil {
			s.Fatal("Failed to create Shill property watcher: ", err)
		}

		// Set up a second AP with the same SSID.
		s.Log("Configuring AP 1")
		ap1, err := tf.ConfigureAP(ctx, apOpts1, nil)
		if err != nil {
			s.Fatal("Failed to configure AP 1: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap1); err != nil {
				s.Error("Failed to deconfig AP 1: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
		defer cancel()

		// Get the name and MAC address of the DUT WiFi interface.
		clientIface, err := tf.ClientInterface(ctx)
		if err != nil {
			s.Fatal("Unable to get DUT interface name: ", err)
		}
		clientMAC, err := tf.ClientHardwareAddr(ctx)
		if err != nil {
			s.Fatal("Unable to get DUT MAC address: ", err)
		}

		// Flush all scanned BSS from wpa_supplicant so that test behavior is consistent.
		s.Log("Flushing BSS cache")
		if err := tf.FlushBSS(ctx, clientIface, 0); err != nil {
			s.Fatal("Failed to flush BSS list: ", err)
		}

		// Wait for roamBSSID to be discovered if waitForScan is set.
		if waitForScan {
			s.Logf("Waiting for roamBSSID: %s", roamBSSID)
			if err := tf.DiscoverBSSID(ctx, roamBSSID, clientIface, []byte(testSSID)); err != nil {
				s.Fatal("Unable to discover roam BSSID: ", err)
			}
		}

		// Send BSS Transition Management Request to client.
		s.Logf("Sending BSS Transition Management Request from AP %s to DUT %s", fromBSSID, clientMAC)
		if err := ap0.SendBSSTMRequest(ctx, clientMAC, roamBSSID); err != nil {
			s.Fatal("Failed to send BSS TM Request: ", err)
		}

		// Wait for the DUT to roam to the second AP, then assert that there was
		// no disconnection during roaming.
		s.Log("Waiting for roaming")
		monitorResult, err := waitForProps()
		if err != nil {
			s.Fatal("Failed to roam within timeout: ", err)
		}
		for _, ph := range monitorResult {
			if ph.Name == shillconst.ServicePropertyIsConnected {
				if !ph.Value.(bool) {
					s.Fatal("Failed to stay connected during the roaming process")
				}
			}
		}

		// Just for good measure make sure we're properly connected.
		s.Log("Verifying connection to AP 1")
		if err := tf.VerifyConnection(ctx, ap1); err != nil {
			s.Fatal("DUT: failed to verify connection: ", err)
		}
	}

	// Before sending the BSS TM request, run a scan and make sure the DUT
	// has seen the second AP. In that case, the DUT will typically re-use
	// the result of the scan when receiving the request instead of probing
	// the second AP.
	if !s.Run(ctx, "waitForScan=true", func(ctx context.Context, s *testing.State) {
		runTest(ctx, s, true)
	}) {
		return
	}

	// After setting up both APs, immediately send the BSS TM Request before
	// the DUT has scanned and noticed the second AP (at least in the
	// majority of test runs). Instead of relying on the result of a previous
	// scan, the DUT will probe for the second AP when receiving the
	// transition request.
	if !s.Run(ctx, "waitForScan=false", func(ctx context.Context, s *testing.State) {
		runTest(ctx, s, false)
	}) {
		return
	}
}
