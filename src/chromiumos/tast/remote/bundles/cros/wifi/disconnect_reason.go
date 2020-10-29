// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/network/iw"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

type disconnectTrigger int

type disReason struct {
	dt disconnectTrigger
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
				Val: disReason{
					dt: dtAPGone,
				},
			}, {
				Name: "ap_send_chan_switch",
				Val: disReason{
					dt: dtAPSendChannelSwitch,
				},
			}, {
				Name: "deauth_client",
				Val: disReason{
					dt: dtDeauthClient,
				},
			}, {
				Name: "disable_client_wifi",
				Val: disReason{
					dt: dtDisableClientWiFi,
				},
			}, {
				Name: "switch_ap",
				Val: disReason{
					dt: dtSwitchAP,
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

	const (
		initialChannel   = 64
		alternateChannel = 1
		maxRetry         = 5

		// Previous authentication no longer valid.
		previousAuthenticationInvalid = 2
		// Deauthenticated because sending STA is leaving (or has left) IBSS or ESS.
		deauthSTALeaving = 3
		// Negative value indicates locally generated disconnection.
		lgDeauthSTALeaving = -3
		// Disassociated due to inactivity.
		lgDisassociatedInactivity = -4
	)

	// Configure an AP and returns a callback to deconfigure the AP and an error object.
	// Note that it directly uses s and tf from the outer scope.
	configureAP := func(ctx context.Context) (*wificell.APIface, func() error, error) {
		options := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(initialChannel), hostapd.HTCaps(hostapd.HTCapHT20)}
		ap, err := tf.ConfigureAP(ctx, options, nil)
		if err != nil {
			return nil, nil, err
		}
		deferFunc := func() error {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				return err
			}
			return nil
		}
		return ap, deferFunc, nil
	}

	ap1, deconfigAP1, err := configureAP(ctx)
	if err != nil {
		s.Fatal("Failed to set up AP: ", err)
	}
	defer func() {
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

	reasonRecv, err := tf.DisconnectReason(ctx)
	if err != nil {
		s.Fatal("Failed to create a Disconnect Reason watcher: ", err)
	}

	var expectedDisconnectReason int32
	trigger := s.Param().(disReason)
	switch trigger.dt {
	case dtAPGone:
		if err := deconfigAP1(); err != nil {
			s.Fatal("Failed to deconfig the AP: ", err)
		}
		deconfigAP1 = nil
		expectedDisconnectReason = deauthSTALeaving
	case dtAPSendChannelSwitch:
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

		ctxForResetingPowersaveMode := ctx
		ctx, cancel = ctxutil.Shorten(ctx, time.Second)
		defer cancel()
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
			}(ctxForResetingPowersaveMode)
			s.Log("Disabling power save in the test")
			if err := iwr.SetPowersaveMode(ctx, iface, false); err != nil {
				s.Fatal("Failed to turn off powersave: ", err)
			}
		}

		ctxForCloseFrameSender := ctx
		ctx, cancel = tf.Router().ReserveForCloseFrameSender(ctx)
		defer cancel()
		sender, err := tf.Router().NewFrameSender(ctx, ap1.Interface())
		if err != nil {
			s.Fatal("Failed to create frame sender: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.Router().CloseFrameSender(ctx, sender); err != nil {
				s.Error("Failed to close frame sender: ", err)
			}
		}(ctxForCloseFrameSender)

		ew, err := iw.NewEventWatcher(ctx, s.DUT())
		if err != nil {
			s.Fatal("Failed to start iw.EventWatcher: ", err)
		}
		defer ew.Stop(ctx)

		// Action frame might be lost, give it some retries.
		csaDisEventDetected := false
		for i := 0; i < maxRetry; i++ {
			s.Logf("Try sending channel switch frame %d", i)
			if err := sender.Send(ctx, framesender.TypeChannelSwitch, alternateChannel); err != nil {
				s.Fatal("Failed to send channel switch frame: ", err)
			}
			// The frame might need some time to reach DUT, wait for a few seconds.
			wCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			// TODO(b/154879577): Find some way to know if DUT supports
			// channel switch, and only wait for the proper event.
			_, err := ew.WaitByType(wCtx, iw.EventTypeChanSwitch, iw.EventTypeDisconnect)
			if err == context.DeadlineExceeded {
				// Retry if deadline exceeded.
				continue
			}
			if err != nil {
				s.Fatal("Failed to wait for iw event: ", err)
			}
			// Channel switch or client disconnection detected.
			csaDisEventDetected = true
			break
		}
		if !csaDisEventDetected {
			s.Fatal("Client failed to disconnect or switch channel")
		}
		expectedDisconnectReason = lgDisassociatedInactivity
	case dtDeauthClient:
		clientHWAddr, err := tf.ClientHardwareAddr(ctx)
		if err != nil {
			s.Fatal("Failed to get the DUT MAC address: ", err)
		}
		if err := ap1.DeauthenticateClient(ctx, clientHWAddr); err != nil {
			s.Fatal("Failed to deauthenticate the DUT: ", err)
		}
		expectedDisconnectReason = previousAuthenticationInvalid
	case dtDisableClientWiFi:
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
		expectedDisconnectReason = lgDeauthSTALeaving
	case dtSwitchAP:
		// Set up the second AP.
		ap2, deconfigAP2, err := configureAP(ctx)
		if err != nil {
			s.Fatal("Failed to set up AP: ", err)
		}
		defer func() {
			if deconfigAP2 != nil {
				if err := deconfigAP2(); err != nil {
					s.Error("Failed to deconfig the AP: ", err)
				}
			}
		}()
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap2)
		defer cancel()
		if _, err := tf.ConnectWifiAP(ctx, ap2); err != nil {
			s.Fatal("DUT: failed to connect to WiFi: ", err)
		}
		expectDisconnectErr = true
		expectedDisconnectReason = lgDeauthSTALeaving
	}

	// Wait for a disconnect reason code from wpa_supplicant.
	reason, err := reasonRecv()
	if err != nil {
		s.Fatal("Failed to wait for the disconnect reason: ", err)
	}

	// Verify the disconnect reason code.
	if reason != expectedDisconnectReason {
		s.Fatalf("Unexpected disconnect reason; got %d, want %d", reason, expectedDisconnectReason)
	}

	s.Logf("DUT: IEEE802.11 reason code for disconnect: %d", reason)

}
