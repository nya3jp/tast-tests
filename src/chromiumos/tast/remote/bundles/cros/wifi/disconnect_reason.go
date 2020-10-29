// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

const (
	apGone              = "AP Gone"
	apSendChannelSwitch = "AP send channel switch"
	deauthClient        = "Deauthenticate clinet"
	disableClientWifi   = "Disable client wifi"
	switchAP            = "Switch AP"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        DisconnectReason,
		Desc:        "Verify the DUT disconnects from an AP and read (but not verify) the supplicant DisconnectReason for various scenarios",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_cq"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
		Params: []testing.Param{
			{
				Name: "ap_gone",
				Val:  apGone,
			}, {
				Name: "ap_send_chan_switch",
				Val:  apSendChannelSwitch,
			}, {
				Name: "deauth_client",
				Val:  deauthClient,
			}, {
				Name: "disable_client_wifi",
				Val:  disableClientWifi,
			}, {
				Name: "switch_ap",
				Val:  switchAP,
			},
		},
	})
}

func DisconnectReason(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	const (
		initialChannel   = 64
		alternateChannel = 1
	)

	// Configure an AP and returns a shortened ctx, a callback to deconfigure the AP and an
	// error object. Note that it directly uses s and tf from the outer scope.
	configureAP := func(ctx context.Context) (context.Context, *wificell.APIface, func(context.Context), error) {
		options := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(initialChannel), hostapd.HTCaps(hostapd.HTCapHT20)}
		ap, err := tf.ConfigureAP(ctx, options, nil)
		if err != nil {
			return ctx, nil, nil, err
		}
		sCtx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		deferFunc := func(ctx context.Context) {
			if ap != nil {
				// ap is already deconfigured.
				return
			}
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig AP: ", err)
			}
			cancel()
		}
		return sCtx, ap, deferFunc, nil
	}

	sCtx, ap1, deconfig, err := configureAP(ctx)
	if err != nil {
		s.Fatal("Failed to set up AP: ", err)
	}
	defer deconfig(ctx)
	ctx = sCtx

	// Connect to the initial AP.
	if _, err := tf.ConnectWifiAP(ctx, ap1); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DisconnectWifi(ctx); err != nil {
			// Do not fail on this error as we're triggering some
			// disconnection in this test and the service can be
			// inactive at this point.
			s.Log("Failed to disconnect WiFi (The service might have been already idle, as the test is triggering some disconnection): ", err)
		}
		// Explicitly delete service entries here because it could have
		// no active service here so calling tf.CleanDisconnectWifi()
		// would fail.
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap1.Config().SSID)}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap1.Config().SSID, err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.VerifyConnection(ctx, ap1); err != nil {
		s.Fatal("DUT: failed to verify connection: ", err)
	}

	reasonRecver, err := tf.DisconnectReason(ctx)
	if err != nil {
		s.Fatal("Failed to create a Disconnect Reason watcher: ", err)
	}

	switch val := s.Param().(string); val {
	case apGone:
		if err := tf.DeconfigAP(ctx, ap1); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	case apSendChannelSwitch:
		sender, err := tf.Router().NewFrameSender(ctx, ap1.Interface())
		if err != nil {
			s.Fatal("Failed to create frame sender: ", err)
		}
		defer func(dCtx context.Context) {
			if err := tf.Router().CloseFrameSender(dCtx, sender); err != nil {
				s.Error("Failed to close frame sender: ", err)
			}
		}(ctx)
		ctx, cancel = tf.Router().ReserveForCloseFrameSender(ctx)
		defer cancel()
		if err := sender.Send(ctx, framesender.TypeChannelSwitch, alternateChannel); err != nil {
			s.Fatal("Failed to send channel switch frame: ", err)
		}
	case deauthClient:
		clientHWAddr, err := tf.ClientHardwareAddr(ctx)
		if err != nil {
			s.Fatal("Failed to get the DUT MAC address: ", err)
		}
		if err := ap1.DeauthenticateClient(ctx, clientHWAddr); err != nil {
			s.Fatal("Failed to disconnect WiFi: ", err)
		}
	case disableClientWifi:
		// TODO: delete this and use the function tf.SetWifiEnabled in the
		// pending CL: chromium:2485885.
		setWifiEnabled := func(ctx context.Context, enabled bool) error {
			req := &network.SetWifiEnabledRequest{Enabled: enabled}
			_, err := tf.WifiClient().SetWifiEnabled(ctx, req)
			return err
		}
		if err := setWifiEnabled(ctx, false); err != nil {
			s.Fatal("DUT: failed to disable the wifi, err: ", err)
		}
		defer func(ctx context.Context) {
			if err := setWifiEnabled(ctx, true); err != nil {
				s.Fatal("DUT: failed to enable the wifi, err: ", err)
			}
		}(ctx)
	case switchAP:
		// Set up the second AP.
		sCtx, ap2, deconfig, err := configureAP(ctx)
		if err != nil {
			s.Fatal("Failed to set up AP: ", err)
		}
		defer deconfig(ctx)
		ctx = sCtx
		if _, err := tf.ConnectWifiAP(ctx, ap2); err != nil {
			s.Fatal("DUT: failed to connect to WiFi: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi: ", err)
			}
		}(ctx)
	}

	// Wait for a disconnect reason code from wpa_supplicant.
	reason, err := reasonRecver()
	if err != nil {
		s.Fatal("Failed to wait for the disconnect reason: ", err)
	}
	s.Logf("DUT: IEEE802.11 reason code for disconnect: %d", reason)
}
