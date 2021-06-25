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
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CSALeaveChannel,
		Desc: "Verifies that DUT will move off-channel after the AP sends a Spectrum Management action frame with a Channel Move element",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func CSALeaveChannel(ctx context.Context, s *testing.State) {
	// Note: Not all clients support CSA, but they generally should at least try
	// to disconnect from the AP which is what the test expects to see.

	tf := s.FixtValue().(*wificell.TestFixture)

	legacyRouter, err := tf.LegacyRouter()
	if err != nil {
		s.Fatal("Failed to get legacy router: ", err)
	}

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
		var cancel context.CancelFunc
		ctx, cancel = ctxutil.Shorten(ctx, time.Second)
		defer cancel()

		s.Log("Disabling power save in the test")
		if err := iwr.SetPowersaveMode(ctx, iface, false); err != nil {
			s.Fatal("Failed to turn off powersave: ", err)
		}
	}

	apOps := []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nMixed),
		hostapd.Channel(64),
		hostapd.HTCaps(hostapd.HTCapHT20),
	}
	ap, err := tf.ConfigureAP(ctx, apOps, nil)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}(ctx)
	s.Log("AP setup done")
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DisconnectWifi(ctx); err != nil {
			// Do not fail on this error as we're triggering some
			// disconnection in this test and the service can be
			// inactive at this point.
			s.Log("Failed to disconnect WiFi: ", err)
		}
		req := &wifi.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().SSID, err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	s.Log("Connected")

	// Assert connection.
	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from DUT: ", err)
	}

	sender, err := legacyRouter.NewFrameSender(ctx, ap.Interface())
	if err != nil {
		s.Fatal("Failed to create frame sender: ", err)
	}
	defer func(dCtx context.Context) {
		if err := legacyRouter.CloseFrameSender(dCtx, sender); err != nil {
			s.Error("Failed to close frame sender: ", err)
		}
	}(ctx)
	ctx, cancel = legacyRouter.ReserveForCloseFrameSender(ctx)
	defer cancel()

	ew, err := iw.NewEventWatcher(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to start iw.EventWatcher: ", err)
	}
	defer ew.Stop(ctx)

	const maxRetry = 5
	const alterChannel = 36
	// Action frame might be lost, give it some retries.
	for i := 0; i < maxRetry; i++ {
		s.Logf("Try sending channel switch frame %d", i)
		if err := sender.Send(ctx, framesender.TypeChannelSwitch, alterChannel); err != nil {
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
		// Channel switch or client disconnection detected, test passed.
		return
	}
	s.Fatal("Client failed to disconnect or switch channel")
}
