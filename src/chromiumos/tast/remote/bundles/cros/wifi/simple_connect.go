// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"

	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type simpleConnectTestcase struct {
	apOpts []ap.Option
	// If unassigned, use default security config: open network.
	secConfFac security.ConfigFactory
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        SimpleConnect,
		Desc:        "Verifies that DUT can connect to the host via AP in different WiFi configuration",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.Wifi"},
		Vars:        []string{"router"},
		Params: []testing.Param{
			{
				// Verifies that DUT can connect to an open 802.11a network on channels 48, 64.
				Name: "80211a",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211a), ap.Channel(48)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211a), ap.Channel(64)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11b network on channels 1, 6, 11.
				Name: "80211b",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211b), ap.Channel(1)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211b), ap.Channel(6)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211b), ap.Channel(11)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11g network on channels 1, 6, 11.
				Name: "80211g",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(6)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(11)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on 2.4GHz channels 1, 6, 11 with a channel width of 20MHz.
				Name: "80211n24ht20",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(1), ap.HTCaps(ap.HTCapHT20)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT20)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(11), ap.HTCaps(ap.HTCapHT20)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on 2.4GHz channel 6 with a channel width of 40MHz.
				Name: "80211n24ht40",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT40)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on 5GHz channel 48 with a channel width of 20MHz.
				Name: "80211n5ht20",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on 5GHz channel 48
				// (40MHz channel with the second 20MHz chunk of the 40MHz channel on the channel below the center channel).
				Name: "80211n5ht40",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT40Minus)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 60 with a channel width of 20MHz.
				Name: "80211acvht20",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acPure), ap.Channel(60), ap.HTCaps(ap.HTCapHT20),
						ap.VHTChWidth(ap.VHTChWidth20Or40),
					}},
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 120 with a channel width of 40MHz.
				Name: "80211acvht40",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acPure), ap.Channel(120), ap.HTCaps(ap.HTCapHT40),
						ap.VHTChWidth(ap.VHTChWidth20Or40),
					}},
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on 5GHz channel 36 with center channel of 42 and channel width of 80MHz.
				Name: "80211acvht80mixed",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acMixed), ap.Channel(36), ap.HTCaps(ap.HTCapHT40Plus),
						ap.VHTCaps(ap.VHTCapSGI80), ap.VHTCenterChannel(42), ap.VHTChWidth(ap.VHTChWidth80),
					}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 157 with center channel of 155 and channel width of 80MHz.
				// The router is forced to use 80 MHz wide rates only.
				Name: "80211acvht80pure",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acPure), ap.Channel(157), ap.HTCaps(ap.HTCapHT40Plus),
						ap.VHTCaps(ap.VHTCapSGI80), ap.VHTCenterChannel(155), ap.VHTChWidth(ap.VHTChWidth80),
					}},
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			}, {
				// Verifies that DUT can connect to an hidden network on 2.4GHz and 5GHz channels.
				Name: "hidden",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(6), ap.Hidden()}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(36), ap.HTCaps(ap.HTCapHT20), ap.Hidden()}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20), ap.Hidden()}},
				},
			},
		},
	})
}

func SimpleConnect(fullCtx context.Context, s *testing.State) {
	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(fullCtx, s.DUT(), s.RPCHint(), router)
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

	testOnce := func(fullCtx context.Context, s *testing.State, options []ap.Option, fac security.ConfigFactory) {
		ap, err := tf.ConfigureAP(fullCtx, options, fac)
		if err != nil {
			s.Fatal("Failed to configure ap, err: ", err)
		}
		defer func() {
			if err := tf.DeconfigAP(fullCtx, ap); err != nil {
				s.Error("Failed to deconfig ap, err: ", err)
			}
		}()
		ctx, cancel := tf.ReserveForDeconfigAP(fullCtx, ap)
		defer cancel()
		s.Log("AP setup done")

		if err := tf.ConnectWifi(ctx, ap); err != nil {
			s.Fatal("Failed to connect to WiFi, err: ", err)
		}
		defer func() {
			if err := tf.DisconnectWifi(fullCtx); err != nil {
				s.Error("Failed to disconnect WiFi, err: ", err)
			}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(fullCtx, &network.SSID{Ssid: ap.Config().Ssid}); err != nil {
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
			testOnce(ctx, s, tc.apOpts, tc.secConfFac)
		}
		if !s.Run(ctx, fmt.Sprintf("Testcase #%d", i), subtest) {
			// Stop if any sub-test failed.
			return
		}
	}
	s.Log("Tearing down")
}
