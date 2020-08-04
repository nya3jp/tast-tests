// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

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

func init() {
	testing.AddTest(&testing.Test{
		Func:        CSADisconnect,
		Desc:        "Verifies that DUT can still connect to the AP when it is disconnected right after receiving a CSA message. This is to make sure the MAC 80211 queues are not stuck after those two events",
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
	// Note: Not all clients support CSA, but they generally should at least try
	// to disconnect from the AP which is what the test expects to see.

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

	connectAP := func(ctx context.Context, channel int) (context.Context, *wificell.APIface, func(context.Context), error) {
		s.Logf("Setting up the AP on channel %d", channel)
		// [?]: disable autoconnect.
		apOps := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nMixed), hostapd.Channel(channel), hostapd.HTCaps(hostapd.HTCapHT20)}
		ap, err := tf.ConfigureAP(ctx, apOps, nil)
		if err != nil {
			return ctx, nil, nil, err
		}

		if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
			return ctx, nil, nil, err
		}

		// Assert connection.
		if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
			return ctx, nil, nil, err
		}

		sCtx, cancelD := tf.ReserveForDeconfigAP(ctx, ap)
		rCtx, cancelC := ctxutil.Shorten(sCtx, 5*time.Second)
		cancel()
		deferFunc := func(ctx context.Context) {
			s.Logf("Deconfiguring the AP on channel %d", channel)
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig AP: ", err)
			}
			cancelD()

			if err := tf.DisconnectWifi(ctx); err != nil {
				// Do not fail on this error as we're triggering some
				// disconnection in this test and the service can be
				// inactive at this point.
				s.Log("Failed to disconnect WiFi: ", err)
			}
			req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
				s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().SSID, err)
			}
			cancelC()
		}
		return rCtx, ap, deferFunc, nil

	}

	rCtx1, ap1, disconnect1, err := connectAP(ctx, 64)
	if err != nil {
		s.Fatal("Failed to set up and connect AP: ", err)
	}
	defer disconnect1(ctx)
	ctx = rCtx1

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

	evLog, err := iw.NewEventLogger(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to start iw.EventLogger: ", err)
	}
	defer evLog.Stop(ctx)

	const maxRetry = 5
	const alterChannel = 36
	// Action frame might be lost, give it some retries.
	for i := 0; i < maxRetry; i++ {
		s.Logf("Try sending channel switch frame %d", i)
		if err := sender.Send(ctx, framesender.TypeChannelSwitch, alterChannel); err != nil {
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
				s.Log("Client disconnection detected")
				return nil
			}
			return errors.New("no disconnection or channel switch detected")
		}, &testing.PollOptions{
			Timeout:  3 * time.Second,
			Interval: 200 * time.Millisecond,
		}); err == nil {
			// Verified, return.
			return
		}
	}
	s.Fatal("Client failed to disconnect or switch channel")

	if s.Param().(bool) {
		// Client initiated disconnect.
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Fatal("Failed to disconnect WiFi: ", err)
		}
	} else {
		// Router initiated disconnect.
		s.Log("TODO")
	}

	if err := tf.AssureDisconnect(ctx, 20*time.Second); err != nil {
		s.Fatalf("DUT: failed to disconnect in %s: %v", 20*time.Second, err)
	}

	rCtx2, _, disconnect2, err := connectAP(ctx, alterChannel)
	if err != nil {
		s.Fatal("Failed to set up and connect AP: ", err)
	}
	defer disconnect2(ctx)
	ctx = rCtx2

}
