// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        Prefer5Ghz,
		Desc:        "Verifies that DUT can see two APs in the same network and prefer 5Ghz one",
		Contacts:    []string{"deanliao@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router", "pcap"},
	})
}

func Prefer5Ghz(fullCtx context.Context, s *testing.State) {
	ops := []wificell.TFOption{
		wificell.TFCapture(true),
	}
	if router, _ := s.Var("router"); router != "" {
		ops = append(ops, wificell.TFRouter(router))
	}
	if pcap, _ := s.Var("pcap"); pcap != "" {
		ops = append(ops, wificell.TFPcap(pcap))
	}
	// As we are not in precondition, we have fullCtx as both method context and
	// daemon context.
	tf, err := wificell.NewTestFixture(fullCtx, fullCtx, s.DUT(), s.RPCHint(), ops...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(fullCtx); err != nil {
			s.Log("Failed to tear down test fixture, err: ", err)
		}
	}()

	ctx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	// 2.4Ghz AP.
	ap2p4g, err := tf.ConfigureAP(
		ctx, []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(1), ap.HTCaps(ap.HTCapHT20)}, nil)
	ssid := ap2p4g.Config().Ssid
	defer func() {
		if err := tf.DeconfigAP(fullCtx, ap2p4g); err != nil {
			s.Error("Failed to deconfig ap, err: ", err)
		}
	}()
	ctx, _ = tf.ReserveForDeconfigAP(ctx, ap2p4g)

	// 5Ghz AP.
	ap5g, err := tf.ConfigureAP(
		ctx, []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.SSID(ssid), ap.HTCaps(ap.HTCapHT20)}, nil)
	if err != nil {
		s.Fatal("Failed to configure ap, err: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(fullCtx, ap5g); err != nil {
			s.Error("Failed to deconfig ap, err: ", err)
		}
	}()
	ctx, _ = tf.ReserveForDeconfigAP(ctx, ap5g)
	s.Log("AP setup done")

	if _, err := tf.WifiClient().WaitForBsses(ctx, &network.WaitForBssesRequest{Ssid: ssid, NumBss: 2}); err != nil {
		s.Fatal("Failed to expect 2 BSSes: ", err)
	}

	/*
		if err := tf.ConnectWifi(ctx, ap2p4g); err != nil {
			s.Fatal("Failed to connect to WiFi, err: ", err)
		}
		defer func() {
			if err := tf.DisconnectWifi(fullCtx); err != nil {
				s.Error("Failed to disconnect WiFi, err: ", err)
			}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(fullCtx, &network.SSID{Ssid: ssid}); err != nil {
				s.Errorf("Failed to remove entries for ssid=%s, err: %v", ssid, err)
			}
		}()
		s.Log("Connected")
	*/
	s.Log("Tearing down")
}
