// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/errors"
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

	if err := tf.ConnectWifi(ctx, ap2p4g); err != nil {
		s.Fatal("Failed to connect to WiFi, err: ", err)
	}
	defer func() {
		if err := tf.DisconnectWifi(fullCtx); err != nil {
			s.Error("Failed to disconnect WiFi, err: ", err)
		}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(fullCtx, &network.DeleteEntriesForSSIDRequest{Ssid: ssid}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ssid, err)
		}
	}()
	s.Log("Connected")

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get the client's WiFi interface: ", err)
	}

	// Take interface ownership from shill.
	if err := tf.ToggleClientInterface(ctx, iface, false); err != nil {
		s.Fatal("Failed to diable Wifi: ", err)
	}
	defer func() {
		if err := tf.ToggleClientInterface(fullCtx, iface, true); err != nil {
			s.Error("Failed to diable Wifi: ", err)
		}
	}()

	cliConn := tf.ClientConn()
	if cliConn == nil {
		s.Fatal("Failed to obtain client Conn()")
	}
	if err := cliConn.Command("ip", "link", "set", iface, "up").Run(ctx); err != nil {
		s.Fatalf("Failed to bring up %s: %s", iface, err)
	}

	iwr := tf.IwRunner()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		scanResult, err := iwr.ScanDump(ctx, iface)
		if err != nil {
			s.Fatal("Failed to perform iw scan: ", err)
		}
		bssCount := 0
		ssidMap := make(map[string]bool)
		for _, d := range scanResult {
			// testing.ContextLogf(ctx, "BSS: %s  Freq: %d  SSID: %s", data.BSS, data.Frequency, data.SSID)
			if !ssidMap[d.SSID] {
				ssidMap[d.SSID] = true
			}
			if d.SSID == ssid {
				bssCount++
				testing.ContextLogf(ctx, "Added BSS: %s  Freq: %d  SSID: %s", d.BSS, d.Frequency, d.SSID)
			}
		}
		var ssidList []string
		for s := range ssidMap {
			ssidList = append(ssidList, s)
		}
		testing.ContextLogf(ctx, "scan SSIDs: %s", ssidList)
		testing.ContextLogf(ctx, "#BSS: %d", bssCount)
		if bssCount == 2 {
			return nil
		}
		return errors.Errorf("wrong #BSS: got %d; want %d", bssCount, 2)

	}, &testing.PollOptions{
		Timeout:  20 * time.Second,
		Interval: time.Second, // RequestScan is spammy, but shill handles that for us.
	}); err != nil {
		s.Fatal("Failed to expect #BSS: ", err)
	}

	s.Log("Tearing down")
}
