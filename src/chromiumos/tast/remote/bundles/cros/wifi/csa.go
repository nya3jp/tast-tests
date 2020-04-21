// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/remote/network/iw"
	remote_ping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        CSA,
		Desc:        "Verifies that DUT will move off-channel after the AP sends a Spectrum Management action frame with a Channel Move element",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router", "pcap"},
	})
}

func CSA(ctx context.Context, s *testing.State) {
	// Note: Not all clients support CSA, but they generally should at least try
	// to disconnect from the AP which is what the test expects to see.

	var ops []wificell.TFOption
	if router, _ := s.Var("router"); router != "" {
		ops = append(ops, wificell.TFRouter(router))
	}
	if pcap, _ := s.Var("pcap"); pcap != "" {
		ops = append(ops, wificell.TFPcap(pcap))
	}
	tf, err := wificell.NewTestFixture(ctx, ctx, s.DUT(), s.RPCHint(), ops...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func(dCtx context.Context) {
		if err := tf.Close(dCtx); err != nil {
			s.Log("Failed to tear down test fixture: ", err)
		}
	}(ctx)

	ctx, cancel := tf.ReserveForClose(ctx)
	defer cancel()

	apOps := []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nMixed),
		hostapd.Channel(64),
		hostapd.HTCaps(hostapd.HTCapHT20),
	}
	ap, err := tf.ConfigureAP(ctx, apOps, nil)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	defer func(dCtx context.Context) {
		if err := tf.DeconfigAP(dCtx, ap); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}(ctx)
	s.Log("AP setup done")

	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	if err := tf.ConnectWifi(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(dCtx context.Context) {
		if err := tf.DisconnectWifi(dCtx); err != nil {
			// Do not fail on this error as we're triggering some
			// disconnection in this test and the service can be
			// inactive at this point.
			s.Log("Failed to disconnect WiFi: ", err)
		}
		req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().Ssid)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(dCtx, req); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().Ssid, err)
		}
	}(ctx)
	s.Log("Connected")

	// Assert connection.
	if err := tf.PingFromDUT(ctx); err != nil {
		s.Fatal("Failed to ping from DUT: ", err)
	}

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

	pr := remote_ping.NewRunner(s.DUT().Conn())
	pingOps := []ping.Option{
		ping.Count(3),
		ping.Interval(1),
	}

	const maxRetry = 10
	const alterChannel = 36

	evLog, err := iw.NewEventLogger(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to start iw.EventLogger: ", err)
	}
	defer evLog.Stop(ctx)

	for i := 0; i < maxRetry; i++ {
		s.Logf("Try sending channel switch frame %d", i)
		if err := sender.Send(ctx, framesender.TypeChannelSwitch, alterChannel); err != nil {
			s.Fatal("Failed to send channel switch frame: ", err)
		}

		if len(evLog.EventsByType(iw.EventTypeDisconnect)) > 0 {
			s.Log("Client disconnection detected")
			return
		}

		s.Log("Ping server to check connectivity")
		res, err := pr.Ping(ctx, ap.ServerIP().String(), pingOps...)
		if err != nil {
			s.Fatal("Failed to ping server: ", err)
		}
		s.Logf("Ping loss: %f", res.Loss)
		if res.Loss > 60 {
			s.Log("Ping loss threshold reached")
			return
		}
	}
	s.Fatal("Client never lost connectivity")
}
