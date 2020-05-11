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

	// Set up a 2.4GHz AP.
	ap2p4g, err := tf.ConfigureAP(
		ctx, []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(1), ap.HTCaps(ap.HTCapHT20)}, nil)
	ssid := ap2p4g.Config().Ssid
	defer func() {
		if err := tf.DeconfigAP(fullCtx, ap2p4g); err != nil {
			s.Error("Failed to deconfig ap, err: ", err)
		}
	}()
	ctx, _ = tf.ReserveForDeconfigAP(ctx, ap2p4g)

	// Set up a 5GHz AP.
	channel5g := 48
	freq5g, err := ap.ChannelToFrequency(channel5g)
	if err != nil {
		s.Fatalf("Failed to look up frequency for channel %d: %s", channel5g, err)
	}
	ap5g, err := tf.ConfigureAP(
		ctx, []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(channel5g), ap.SSID(ssid), ap.HTCaps(ap.HTCapHT20)}, nil)
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

	serviceReq := &network.GetShillServiceRequest{
		Type: "wifi",
		Ssid: ssid}
	serviceResp, err := tf.WifiClient().GetShillService(ctx, serviceReq)
	if err != nil {
		s.Fatal("Failed to get shill service, err: ", err)
	}
	if len(serviceResp.FrequencyList) != 2 {
		s.Fatalf("Got frequency list of SSID %q: %v; want both 2.4GHz and 5GHz signals", ssid, serviceResp.FrequencyList)
	}
	if int(serviceResp.Frequency) != freq5g {
		s.Fatalf("Got frequency of SSID %q: %dGHz; want %dGHz", ssid, serviceResp.Frequency, freq5g)
	}
	testing.ContextLogf(ctx, "Shill picks frequency %dGHz for SSID %q out of frequencies %v", serviceResp.Frequency, ssid, serviceResp.FrequencyList)

	testing.ContextLog(ctx, "Asserting the connection")
	if err := tf.ConnectWifi(ctx, ap2p4g); err != nil {
		s.Fatal("Failed to connect to WiFi, err: ", err)
	}
	defer func() {
		if err := tf.DisconnectWifi(fullCtx); err != nil {
			s.Error("Failed to disconnect WiFi, err: ", err)
		}
		req := &network.DeleteEntriesForSSIDRequest{Ssid: ssid}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(fullCtx, req); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ssid, err)
		}
	}()

	s.Log("Tearing down")
}
