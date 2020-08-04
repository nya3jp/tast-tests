// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/iw"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

const (
	primaryChannel   = 64
	alternateChannel = 36
	maxRetry         = 5
	bssid            = "00:11:22:33:44:55"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        CSADisconnect,
		Desc:        "Verifies that the DUT can still connect to the AP when it is disconnected right after receiving a CSA message. This is to make sure the MAC 80211 queues are not stuck after those two events",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
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

func CSADisconnect(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	// TODO(b/154879577): Currently the action frames sent by FrameSender
	// are not buffered for DTIM so if the DUT is in powersave mode, it
	// cannot receive the action frame and the test will fail.
	// Turn off powersave mode to replicate the behavior of Autotest in
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
		ctx, cancel = ctxutil.Shorten(ctx, time.Second)
		defer cancel()

		s.Log("Disabling power save in the test")
		if err := iwr.SetPowersaveMode(ctx, iface, false); err != nil {
			s.Fatal("Failed to turn off powersave: ", err)
		}
	}

	connectAP := func(ctx context.Context, channel int) (context.Context, *wificell.APIface, func(context.Context), string, error) {
		s.Logf("Setting up the AP on channel %d", channel)
		apOps := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nMixed), hostapd.Channel(channel), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.BSSID(bssid)}
		ap, err := tf.ConfigureAP(ctx, apOps, nil)
		if err != nil {
			return ctx, nil, nil, "", err
		}

		// Disable autoconnect.
		configProps := map[string]interface{}{
			shillconst.ServicePropertyAutoConnect: false,
		}

		resp, err := tf.ConnectWifiAP(ctx, ap, configProps)
		if err != nil {
			return ctx, nil, nil, "", err
		}
		servicePath := resp.ServicePath

		// Assert connection.
		if err := tf.VerifyConnection(ctx, ap); err != nil {
			s.Fatal("Failed to verify connection: ", err)
		}

		sCtx, cancelDeconf := tf.ReserveForDeconfigAP(ctx, ap)
		rCtx, cancelDisconn := tf.ReserveForDisconnect(sCtx)

		deferFunc := func(ctx context.Context) {
			cancelDisconn()
			if err := tf.DisconnectWifi(ctx); err != nil {
				// Do not fail on this error as we're triggering some
				// disconnection in this test and the service can be
				// inactive at this point.
				s.Log("Failed to disconnect WiFi: ", err)
			}
			req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
				s.Logf("Failed to remove entries for ssid=%s, err: %v", ap.Config().SSID, err)
			}

			cancelDeconf()
			if ap == nil {
				// ap is already deconfigured.
				return
			}
			s.Logf("Deconfiguring the AP on channel %d", channel)
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig AP: ", err)
			}
		}

		return rCtx, ap, deferFunc, servicePath, nil
	}

	retCtx, ap, disconnectPrimary, servicePath, err := connectAP(ctx, primaryChannel)
	if err != nil {
		s.Fatalf("Failed to set up and connect AP with primary channel %d: %v", primaryChannel, err)
	}
	defer disconnectPrimary(ctx)
	ctx = retCtx

	sender, err := tf.Router().NewFrameSender(ctx, ap.Interface())
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

	disconnDetected := false
	func(context.Context) {
		evLog, err := iw.NewEventLogger(ctx, s.DUT())
		if err != nil {
			s.Fatal("Failed to start iw.EventLogger: ", err)
		}
		defer evLog.Stop(ctx)

		// Action frame might be lost, give it some retries.
		for i := 0; i < maxRetry; i++ {
			s.Logf("Try sending channel switch frame %d", i)

			if err := sender.Send(ctx, framesender.TypeChannelSwitch, alternateChannel); err != nil {
				s.Fatal("Failed to send channel switch frame: ", err)
			}

			// The frame might need some time to reach DUT, poll for a few seconds.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				// TODO(b/154879577): Find some way to know if DUT supports
				// channel switch, and only wait for the proper event.
				if len(evLog.EventsByType(iw.EventTypeChanSwitch)) > 0 {
					s.Log("Channel switch detected")
					return nil
				}
				if len(evLog.EventsByType(iw.EventTypeDisconnect)) > 0 {
					disconnDetected = true
					s.Log("Client disconnection detected")
					return nil
				}
				return errors.New("no disconnection or channel switch detected")
			}, &testing.PollOptions{
				Timeout:  3 * time.Second,
				Interval: 200 * time.Millisecond,
			}); err == nil {
				// Verified, break.
				break
			}
		}
	}(ctx)

	if !disconnDetected {
		if s.Param().(bool) {
			// Client initiated disconnect.
			if err := tf.DisconnectWifi(ctx); err != nil {
				s.Fatal("Failed to disconnect WiFi: ", err)
			}
		} else {
			// Router initiated disconnect.
			if err := tf.Router().DeauthenticateClient(ctx, ap, bssid); err != nil {
				s.Fatal("Failed to disconnect WiFi: ", err)
			}
			// Wait for DUT to disconnect.
			if err := tf.AssureDisconnect(ctx, servicePath, 20*time.Second); err != nil {
				s.Fatalf("DUT: failed to disconnect in %s: %v", 20*time.Second, err)
			}
		}
	}
	req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
	if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
		s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().SSID, err)
	}
	s.Log("Client Disconnetcted")

	if err := tf.DeconfigAP(ctx, ap); err != nil {
		s.Error("Failed to deconfig AP: ", err)
	}
	ap = nil
	s.Log("AP Disconnetcted")

	_, _, disconnectAlternate, _, err := connectAP(ctx, alternateChannel)
	if err != nil {
		s.Fatalf("Failed to set up and connect AP with alternate channel %d: %v", alternateChannel, err)
	}
	disconnectAlternate(ctx)
}
