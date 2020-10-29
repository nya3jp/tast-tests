// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/wpasupplicant"
	"chromiumos/tast/remote/network/iw"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/framesender"
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
					expectedCode: wpasupplicant.DeauthSTALeaving,
				},
			}, {
				Name: "ap_send_chan_switch",
				Val: disReasonParam{
					dt:           dtAPSendChannelSwitch,
					expectedCode: wpasupplicant.LGDisassociatedInactivity,
				},
			}, {
				Name: "deauth_client",
				Val: disReasonParam{
					dt:           dtDeauthClient,
					expectedCode: wpasupplicant.PreviousAuthenticationInvalid,
				},
			}, {
				Name: "disable_client_wifi",
				Val: disReasonParam{
					dt:           dtDisableClientWiFi,
					expectedCode: wpasupplicant.LGDeauthSTALeaving,
				},
			}, {
				Name: "switch_ap",
				Val: disReasonParam{
					dt:           dtSwitchAP,
					expectedCode: wpasupplicant.LGDeauthSTALeaving,
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

	switch trigger.dt {
	case dtAPGone:
		if err := deconfigAP1(); err != nil {
			s.Fatal("Failed to deconfig the AP: ", err)
		}
		// Setting the deconfigAP1 to nil to avoid double deconfiguring again in the defered function above.
		deconfigAP1 = nil
	case dtAPSendChannelSwitch:
		wCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := sendChannelSwitchAnnouncement(wCtx, tf, ap1, maxRetry, alternateChannel, s.DUT()); err != nil {
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
		if err := tf.SetWifiEnabled(ctx, false); err != nil {
			s.Fatal("DUT: failed to disable the wifi, err: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.SetWifiEnabled(ctx, true); err != nil {
				s.Fatal("DUT: failed to enable the wifi, err: ", err)
			}
		}(ctx)
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

// sendChannelSwitchAnnouncement sends a CSA frame and waits for Client_Diconnection, or ChannelSwitch event.
func sendChannelSwitchAnnouncement(ctx context.Context, tf *wificell.TestFixture, ap *wificell.APIface, maxRetry, alternateChannel int, dut *dut.DUT) error {
	ctxForCloseFrameSender := ctx
	ctx, cancel := tf.Router().ReserveForCloseFrameSender(ctx)
	defer cancel()
	sender, err := tf.Router().NewFrameSender(ctx, ap.Interface())
	if err != nil {
		return errors.Wrap(err, "failed to create frame sender")
	}
	defer func(ctx context.Context) error {
		if err := tf.Router().CloseFrameSender(ctx, sender); err != nil {
			return errors.Wrap(err, "failed to close frame sender")
		}
		return nil
	}(ctxForCloseFrameSender)

	ew, err := iw.NewEventWatcher(ctx, dut)
	if err != nil {
		return errors.Wrap(err, "failed to start iw.EventWatcher")
	}
	defer ew.Stop(ctx)

	// Action frame might be lost, give it some retries.
	for i := 0; i < maxRetry; i++ {
		testing.ContextLogf(ctx, "Try sending channel switch frame %d", i)
		if err := sender.Send(ctx, framesender.TypeChannelSwitch, alternateChannel); err != nil {
			return errors.Wrap(err, "failed to send channel switch frame")
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
			return errors.Wrap(err, "failed to wait for iw event")
		}
		// Channel switch or client disconnection detected.
		return nil
	}

	return errors.New("failed to disconnect client or switch channel")

}
