// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/wpasupplicant"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

type disconnectTrigger int

type disReasonParam struct {
	dt           disconnectTrigger
	expectedCode int32
}

const (
	dtAPGone disconnectTrigger = iota
	dtAPSendChannelSwitch
	dtDeauthClient
	dtDisableClientWiFi
	dtSwitchAP
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        DisconnectReason,
		Desc:        "Verify the DUT disconnects from an AP and verify the supplicant DisconnectReason for various scenarios",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
		Params: []testing.Param{
			{
				Name: "ap_gone",
				Val: disReasonParam{
					dt:           dtAPGone,
					expectedCode: wpasupplicant.DisconnReasonDeauthSTALeaving,
				},
			}, {
				Name: "ap_send_chan_switch",
				Val: disReasonParam{
					dt:           dtAPSendChannelSwitch,
					expectedCode: wpasupplicant.DisconnReasonLGDisassociatedInactivity,
				},
			}, {
				Name: "deauth_client",
				Val: disReasonParam{
					dt:           dtDeauthClient,
					expectedCode: wpasupplicant.DisconnReasonPreviousAuthenticationInvalid,
				},
			}, {
				Name: "disable_client_wifi",
				Val: disReasonParam{
					dt:           dtDisableClientWiFi,
					expectedCode: wpasupplicant.DisconnReasonLGDeauthSTALeaving,
				},
			}, {
				Name: "switch_ap",
				Val: disReasonParam{
					dt:           dtSwitchAP,
					expectedCode: wpasupplicant.DisconnReasonLGDeauthSTALeaving,
				},
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

	trigger := s.Param().(disReasonParam)
	if trigger.dt == dtAPSendChannelSwitch {
		// TODO(b/154879577): Currently the action frames sent by FrameSender
		// are not buffered for DTIM so if the DUT is in power saving mode, it
		// cannot receive the action frame and the test will fail.
		// Turn off power saving mode to replicate the behavior of Autotest in
		// this test for now.
		shortenCtx, restore, err := tf.DisablePowersaveMode(ctx)
		if err != nil {
			s.Fatal("Failed to disable power saving mode: ", err)
		}
		ctx = shortenCtx
		defer func() {
			if err := restore(); err != nil {
				s.Error("Failed to restore initial power saving mode: ", err)
			}
		}()
	}

	const (
		initialChannel   = 64
		alternateChannel = 1
		maxRetry         = 5
	)

	// Configure an AP and returns a callback to deconfigure the AP and an error object.
	configureAP := func(ctx context.Context) (wificell.APIface, func() error, error) {
		options := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(initialChannel), hostapd.HTCaps(hostapd.HTCapHT20)}
		ap, err := tf.ConfigureAP(ctx, options, nil)
		if err != nil {
			return nil, nil, err
		}
		deferFunc := func() error {
			return tf.DeconfigAP(ctx, ap)
		}
		return ap, deferFunc, nil
	}

	ap1, deconfigAP1, err := configureAP(ctx)
	if err != nil {
		s.Fatal("Failed to set up AP: ", err)
	}
	defer func() {
		// Avoid deconfiguring the AP, if the test triggers an early
		// deconfiguration the AP such as in the subtest ap_gone.
		if deconfigAP1 != nil {
			if err := deconfigAP1(); err != nil {
				s.Error("Failed to deconfig the AP: ", err)
			}
		}
	}()
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()

	// Connect to the initial AP.
	ctxForDisconnect := ctx
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	if _, err := tf.ConnectWifiAP(ctx, ap1); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	}
	expectDisconnectErr := false
	defer func(ctx context.Context) {
		if expectDisconnectErr {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi: ", err)
			}
		} else {
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
		}
	}(ctxForDisconnect)

	if err := tf.VerifyConnection(ctx, ap1); err != nil {
		s.Fatal("DUT: failed to verify connection: ", err)
	}

	disconnCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	reasonRecv, err := tf.DisconnectReason(disconnCtx)
	if err != nil {
		s.Fatal("Failed to create a Disconnect Reason watcher: ", err)
	}

	switch trigger.dt {
	case dtAPGone:
		if err := deconfigAP1(); err != nil {
			s.Fatal("Failed to deconfig the AP: ", err)
		}
		// Setting the deconfigAP1 to nil to avoid double deconfiguring again in the deferred function above.
		deconfigAP1 = nil
	case dtAPSendChannelSwitch:
		wCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := tf.SendChannelSwitchAnnouncement(wCtx, ap1, maxRetry, alternateChannel); err != nil {
			s.Fatal("Failed to send channel switch announcement: ", err)
		}
	case dtDeauthClient:
		clientHWAddr, err := tf.ClientHardwareAddr(ctx)
		if err != nil {
			s.Fatal("Failed to get the DUT MAC address: ", err)
		}
		if err := ap1.DeauthenticateClient(ctx, clientHWAddr); err != nil {
			s.Fatal("Failed to deauthenticate the DUT: ", err)
		}
	case dtDisableClientWiFi:
		ctxForEnablingWiFi := ctx
		ctx, cancel = ctxutil.Shorten(ctx, 2*time.Second)
		defer cancel()
		if err := tf.SetWifiEnabled(ctx, false); err != nil {
			s.Fatal("DUT: failed to disable the wifi, err: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.SetWifiEnabled(ctx, true); err != nil {
				s.Fatal("DUT: failed to enable the wifi, err: ", err)
			}
		}(ctxForEnablingWiFi)
	case dtSwitchAP:
		// Set up the second AP.
		ap2, deconfigAP2, err := configureAP(ctx)
		if err != nil {
			s.Fatal("Failed to set up AP: ", err)
		}
		defer func() {
			if err := deconfigAP2(); err != nil {
				s.Error("Failed to deconfig the AP: ", err)
			}
		}()
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap2)
		defer cancel()
		if _, err := tf.ConnectWifiAP(ctx, ap2); err != nil {
			s.Fatal("DUT: failed to connect to WiFi: ", err)
		}
		expectDisconnectErr = true
	}

	// Wait for a disconnect reason code from wpa_supplicant.
	reason, err := reasonRecv()
	if err != nil {
		s.Fatal("Failed to wait for the disconnect reason: ", err)
	}

	// Verify the disconnect reason code.
	if reason != trigger.expectedCode {
		s.Fatalf("Unexpected disconnect reason; got %d, want %d", reason, trigger.expectedCode)
	}

	s.Logf("DUT: IEEE802.11 reason code for disconnect: %d", reason)
}
