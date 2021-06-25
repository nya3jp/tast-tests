// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FunctionalAfterCSA,
		Desc: "Verifies that the DUT can still connect to the AP when it is disconnected right after receiving a CSA message. This is to make sure the MAC 80211 queues are not stuck after receiving CSA and disconnect events consecutively. Refer to crbug.com/408370 for more information to the test description",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
		Params: []testing.Param{
			{
				Name: "client",
				Val:  true,
			}, {
				Name: "router",
				Val:  false,
			},
		},
	})
}

func FunctionalAfterCSA(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	legacyRouter, err := tf.LegacyRouter()
	if err != nil {
		s.Fatal("Failed to get legacy router: ", err)
	}

	const numRounds = 5

	var (
		primaryChannel   = 64
		alternateChannel = 36
	)

	// TODO(b/154879577): Currently the action frames sent by FrameSender
	// are not buffered for DTIM so if the DUT is in power saving mode, it
	// cannot receive the action frame and the test will fail.
	// Turn off power saving mode to replicate the behavior of Autotest in
	// this test for now.
	iwr := remoteiw.NewRemoteRunner(s.DUT().Conn())
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get the client interface: ", err)
	}
	psMode, err := iwr.PowersaveMode(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get the powersave mode: ", err)
	}
	if psMode {
		defer func(ctx context.Context) {
			s.Logf("Restoring power save mode to %t", psMode)
			if err := iwr.SetPowersaveMode(ctx, iface, psMode); err != nil {
				s.Errorf("Failed to restore powersave mode to %t: %v", psMode, err)
			}
		}(ctx)
		var cancel context.CancelFunc
		ctx, cancel = ctxutil.Shorten(ctx, time.Second)
		defer cancel()

		s.Log("Disabling power save in the test")
		if err := iwr.SetPowersaveMode(ctx, iface, false); err != nil {
			s.Fatal("Failed to turn off powersave: ", err)
		}
	}

	clientInitDisconnect := s.Param().(bool)
	csaDisconnectCore := func(ctx context.Context, primaryChannel, alternateChannel int) {
		s.Logf("Setting up the AP on channel %d", primaryChannel)
		apOps := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nMixed), hostapd.Channel(primaryChannel), hostapd.HTCaps(hostapd.HTCapHT20)}
		ap, err := tf.ConfigureAP(ctx, apOps, nil)
		if err != nil {
			s.Fatal("Failed to configure the AP: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Fatal("Failed to deconfig the AP: ", err)
			}
		}(ctx)
		ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()

		s.Log("Connecting to AP")
		// Disable autoconnect.
		configProps := map[string]interface{}{
			shillconst.ServicePropertyAutoConnect: false,
		}
		resp, err := tf.ConnectWifiAP(ctx, ap, wificell.ConnProperties(configProps))
		if err != nil {
			s.Fatal("Failed to connect to WiFi: ", err)
		}
		disconnected := false
		defer func(ctx context.Context) {
			if !disconnected {
				if err := tf.DisconnectWifi(ctx); err != nil {
					s.Error("Failed to disconnect WiFi: ", err)
				}
			}
			req := &wifi.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
				s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().SSID, err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()

		s.Logf("Connected. Sending channel switch frame (channel switch to %d)", alternateChannel)
		sender, err := legacyRouter.NewFrameSender(ctx, ap.Interface())
		if err != nil {
			s.Fatal("Failed to create frame sender: ", err)
		}
		defer func(ctx context.Context) {
			if err := legacyRouter.CloseFrameSender(ctx, sender); err != nil {
				s.Fatal("Failed to close frame sender: ", err)
			}
		}(ctx)
		ctx, cancel = legacyRouter.ReserveForCloseFrameSender(ctx)
		defer cancel()
		if err := sender.Send(ctx, framesender.TypeChannelSwitch, alternateChannel); err != nil {
			s.Fatal("Failed to send channel switch frame: ", err)
		}

		if clientInitDisconnect {
			// Client initiated disconnect.
			if err := tf.DisconnectWifi(ctx); err != nil {
				// Do not fail on this error as CSA could trigger
				// disconnection in this test and the service can be
				// inactive at this point.
				s.Log("Failed to disconnect WiFi: ", err)
			}
		} else {
			clientHWAddr, err := tf.ClientHardwareAddr(ctx)
			if err != nil {
				s.Fatal("Failed to get the DUT MAC address: ", err)
			}
			// Router initiated disconnect.
			if err := ap.DeauthenticateClient(ctx, clientHWAddr); err != nil {
				s.Fatal("Failed to disconnect WiFi: ", err)
			}
			// Wait for DUT to disconnect.
			if err := tf.AssureDisconnect(ctx, resp.ServicePath, 20*time.Second); err != nil {
				s.Fatalf("DUT: failed to disconnect in %s: %v", 20*time.Second, err)
			}
		}
		disconnected = true
	}

	// Run it multiple times to reproduce the race condition that triggers crbug.com/408370.
	// Alternate the AP channel with the CSA announced channel to work around with drivers
	// (Marvell 8897) that disallow reconnecting immediately to the same AP on the same channel
	// after CSA to a different channel.
	for i := 1; i <= numRounds; i++ {
		s.Logf("Run number %d", i)
		csaDisconnectCore(ctx, primaryChannel, alternateChannel)
		// Swap primaryChannel with alternateChannel so we don't configure
		// AP using same channel in back-to-back runs.
		alternateChannel, primaryChannel = primaryChannel, alternateChannel
	}
}
