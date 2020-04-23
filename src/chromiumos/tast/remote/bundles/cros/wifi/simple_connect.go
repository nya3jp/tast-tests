// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"

	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type simpleConnectTestcase struct {
	apOptions []hostapd.Option
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        SimpleConnect,
		Desc:        "Verifies that DUT can connect to the host via AP in different WiFi configuration",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.Wifi"},
		Vars:        []string{"router", "pcap"},
		Params: []testing.Param{
			{
				// Verifies that DUT can connect to an open 802.11a network on channels 48, 64.
				Name: "80211a",
				Val: []simpleConnectTestcase{
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211a), hostapd.Channel(48)}},
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211a), hostapd.Channel(64)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11b network on channels 1, 6, 11.
				Name: "80211b",
				Val: []simpleConnectTestcase{
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(1)}},
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(6)}},
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(11)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11g network on channels 1, 6, 11.
				Name: "80211g",
				Val: []simpleConnectTestcase{
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)}},
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(6)}},
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(11)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on 2.4GHz channels 1, 6, 11 with a channel width of 20MHz.
				Name: "80211n24ht20",
				Val: []simpleConnectTestcase{
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(1), hostapd.HTCaps(hostapd.HTCapHT20)}},
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(6), hostapd.HTCaps(hostapd.HTCapHT20)}},
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(11), hostapd.HTCaps(hostapd.HTCapHT20)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on 2.4GHz channel 6 with a channel width of 40MHz.
				Name: "80211n24ht40",
				Val: []simpleConnectTestcase{
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(6), hostapd.HTCaps(hostapd.HTCapHT40)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on 5GHz channel 48 with a channel width of 20MHz.
				Name: "80211n5ht20",
				Val: []simpleConnectTestcase{
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT20)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on 5GHz channel 48
				// (40MHz channel with the second 20MHz chunk of the 40MHz channel on the channel below the center channel).
				Name: "80211n5ht40",
				Val: []simpleConnectTestcase{
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT40Minus)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 60 with a channel width of 20MHz.
				Name: "80211acvht20",
				Val: []simpleConnectTestcase{
					{[]hostapd.Option{
						hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(60), hostapd.HTCaps(hostapd.HTCapHT20),
						hostapd.VHTChWidth(hostapd.VHTChWidth20Or40),
					}},
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 120 with a channel width of 40MHz.
				Name: "80211acvht40",
				Val: []simpleConnectTestcase{
					{[]hostapd.Option{
						hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(120), hostapd.HTCaps(hostapd.HTCapHT40),
						hostapd.VHTChWidth(hostapd.VHTChWidth20Or40),
					}},
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on 5GHz channel 36 with center channel of 42 and channel width of 80MHz.
				Name: "80211acvht80mixed",
				Val: []simpleConnectTestcase{
					{[]hostapd.Option{
						hostapd.Mode(hostapd.Mode80211acMixed), hostapd.Channel(36), hostapd.HTCaps(hostapd.HTCapHT40Plus),
						hostapd.VHTCaps(hostapd.VHTCapSGI80), hostapd.VHTCenterChannel(42), hostapd.VHTChWidth(hostapd.VHTChWidth80),
					}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 157 with center channel of 155 and channel width of 80MHz.
				// The router is forced to use 80 MHz wide rates only.
				Name: "80211acvht80pure",
				Val: []simpleConnectTestcase{
					{[]hostapd.Option{
						hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(157), hostapd.HTCaps(hostapd.HTCapHT40Plus),
						hostapd.VHTCaps(hostapd.VHTCapSGI80), hostapd.VHTCenterChannel(155), hostapd.VHTChWidth(hostapd.VHTChWidth80),
					}},
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			}, {
				// Verifies that DUT can connect to an hidden network on 2.4GHz and 5GHz channels.
				Name: "hidden",
				Val: []simpleConnectTestcase{
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(6), hostapd.Hidden()}},
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(36), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.Hidden()}},
					{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.Hidden()}},
				},
			},
		},
	})
}

func SimpleConnect(ctx context.Context, s *testing.State) {
	ops := []wificell.TFOption{
		wificell.TFCapture(true),
	}
	if router, _ := s.Var("router"); router != "" {
		ops = append(ops, wificell.TFRouter(router))
	}
	if pcap, _ := s.Var("pcap"); pcap != "" {
		ops = append(ops, wificell.TFPcap(pcap))
	}
	// As we are not in precondition, we have ctx as both method context and
	// daemon context.
	tf, err := wificell.NewTestFixture(ctx, ctx, s.DUT(), s.RPCHint(), ops...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(ctx); err != nil {
			s.Log("Failed to tear down test fixture, err: ", err)
		}
	}()

	testOnce := func(ctx context.Context, s *testing.State, options []hostapd.Option) {
		ap, err := tf.ConfigureAP(ctx, options...)
		if err != nil {
			s.Fatal("Failed to configure ap, err: ", err)
		}
		defer func() {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig ap, err: ", err)
			}
		}()
		s.Log("AP setup done")

		if err := tf.ConnectWifi(ctx, ap); err != nil {
			s.Fatal("Failed to connect to WiFi, err: ", err)
		}
		defer func() {
			if err := tf.DisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi, err: ", err)
			}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.SSID{Ssid: ap.Config().Ssid}); err != nil {
				s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().Ssid, err)
			}
		}()
		s.Log("Connected")

		ping := func(ctx context.Context) error {
			return tf.PingFromDUT(ctx)
		}

		if err := tf.AssertNoDisconnect(ctx, ping); err != nil {
			s.Fatal("Failed to ping from DUT, err: ", err)
		}
		// TODO(crbug.com/1034875): Assert no deauth detected from the server side.
		// TODO(crbug.com/1034875): Maybe some more check on the WiFi capabilities to
		// verify we really have the settings as expected. (ref: crrev.com/c/1995105)
		s.Log("Deconfiguring")
	}

	testcases := s.Param().([]simpleConnectTestcase)
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc.apOptions)
		}
		if !s.Run(ctx, fmt.Sprintf("Testcase #%d", i), subtest) {
			// Stop if any sub-test failed.
			return
		}
	}
	s.Log("Tearing down")
}
