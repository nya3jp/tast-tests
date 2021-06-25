// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/wpasupplicant"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

type disconnectTrigger int

type disReasonParam struct {
	dt            disconnectTrigger
	expectedCodes []int32
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
		Func: DisconnectReason,
		Desc: "Verify the DUT disconnects from an AP and verify the supplicant DisconnectReason for various scenarios",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
		Params: []testing.Param{
			{
				Name: "ap_gone",
				Val: disReasonParam{
					dt: dtAPGone,
					// In this case, the supplicant receives one of the following CMDs:
					// 1- NL80211_CMD_DEAUTHENTICATE due STA leaving.
					// 2- NL80211_CMD_DEAUTHENTICATE due to inactivity.
					// 3- NL80211_CMD_DISCONNECT: happens with full-MAC drivers such as mwifiex.
					expectedCodes: []int32{wpasupplicant.DisconnReasonDeauthSTALeaving,
						wpasupplicant.DisconnReasonLGDisassociatedInactivity,
						wpasupplicant.DisconnReasonUnknown},
				},
			}, {
				Name: "ap_send_chan_switch",
				Val: disReasonParam{
					dt: dtAPSendChannelSwitch,
					// In this case, the supplicant receives one of the following CMDs:
					// 1- NL80211_CMD_DEAUTHENTICATE:
					//    Disconnect reason: reason 4 (DISASSOC_DUE_TO_INACTIVITY) locally_generated=1.
					// 2- NL80211_CMD_DISCONNECT: happens with full-MAC drivers such as mwifiex.
					//    In this case, the disconnect reason is:
					//      a) reason 3 (DEAUTH_LEAVING) locally_generated=1.
					//      b) reason 0 (UNKNOWN) locally_generated=1.
					expectedCodes: []int32{wpasupplicant.DisconnReasonLGDisassociatedInactivity,
						wpasupplicant.DisconnReasonLGDeauthSTALeaving,
						wpasupplicant.DisconnReasonUnknown},
				},
			}, {
				Name: "deauth_client",
				Val: disReasonParam{
					dt:            dtDeauthClient,
					expectedCodes: []int32{wpasupplicant.DisconnReasonPreviousAuthenticationInvalid},
				},
			}, {
				Name: "disable_client_wifi",
				Val: disReasonParam{
					dt:            dtDisableClientWiFi,
					expectedCodes: []int32{wpasupplicant.DisconnReasonLGDeauthSTALeaving},
				},
			}, {
				Name: "switch_ap",
				Val: disReasonParam{
					dt:            dtSwitchAP,
					expectedCodes: []int32{wpasupplicant.DisconnReasonLGDeauthSTALeaving},
				},
			},
		},
	})
}

func DisconnectReason(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

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
	configureAP := func(ctx context.Context) (*wificell.APIface, func() error, error) {
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
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap1)
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
			if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &wifi.DeleteEntriesForSSIDRequest{Ssid: []byte(ap1.Config().SSID)}); err != nil {
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
		// Due to full-MAC drivers not sending a DEAUTH command to wpa supplicant,
		// a disconnection reason is not reported by supplicant.
		if trigger.dt != dtDisableClientWiFi {
			s.Fatal("Failed to wait for the disconnect reason: ", err)
		}

		s.Log("Failed to wait for the disconnect reason: ", err)

		return
	}

	// Verify the disconnect reason code.
	for _, expCode := range trigger.expectedCodes {
		if reason == expCode {
			s.Logf("DUT: received expected IEEE802.11 disconnect reason code: %d", reason)
			return
		}
	}

	s.Fatalf("Unexpected disconnect reason; got %d, want any of %v", reason, trigger.expectedCodes)

}
