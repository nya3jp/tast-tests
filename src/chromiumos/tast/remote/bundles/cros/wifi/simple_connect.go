// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

type simpleConnectTestcase struct {
	apOptions []hostapd.Option
}

type simpleConnectParam struct {
	testcases []simpleConnectTestcase
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        SimpleConnect,
		Desc:        "Verifies that DUT can connect to the host via AP in different WiFi configuration",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:        []string{"group:wificell", "wifi_func"},
		ServiceDeps: []string{"tast.cros.network.Wifi"},
		Vars:        []string{"router"},
		Params: []testing.Param{
			{
				// Verifies that DUT can connect to an open 802.11a network on channels 48, 64.
				Name: "80211a",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211a), hostapd.Channel(48)}},
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211a), hostapd.Channel(64)}},
					},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11b network on channels 1, 6, 11.
				Name: "80211b",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(1)}},
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(6)}},
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(11)}},
					},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11g network on channels 1, 6, 11.
				Name: "80211g",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)}},
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(6)}},
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(11)}},
					},
				},
			}, {
				// Open 802.11n network on 2.4 GHz channels (20MHz channels only).
				Name: "80211n24ht20",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(1), hostapd.HTCaps(hostapd.HTCapHT20)}},
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(6), hostapd.HTCaps(hostapd.HTCapHT20)}},
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(11), hostapd.HTCaps(hostapd.HTCapHT20)}},
					},
				},
			}, {
				// Open 802.11n network on 2.4 GHz channel (40MHz-channel).
				Name: "80211n24ht40",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(6), hostapd.HTCaps(hostapd.HTCapHT40)}},
					},
				},
			}, {
				// Open 802.11n network on 5 GHz channel (20MHz channel only).
				Name: "80211n5ht20",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT20)}},
					},
				},
			}, {
				// Open 802.11n network on 5 GHz channel (40MHz-channel with the second 20MHz chunk of the 40MHz channel on the channel below the center channel).
				Name: "80211n5ht40",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT40Minus)}},
					},
				},
			}, {
				// Open 802.11ac network on channel 60 with a channel width of 20MHz.
				Name: "80211ac5vht20",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						// VHTChWidth40 is the correct configuration option for VHT20 and VHT40
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(60), hostapd.VHTChWidth(hostapd.VHTChWidth40)}},
					},
				},
			}, {
				// Open 802.11ac network on channel 120 with a channel width of 40MHz.
				Name: "80211ac5vht40",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(120), hostapd.HTCaps(hostapd.HTCapHT40), hostapd.VHTChWidth(hostapd.VHTChWidth40)}},
					},
				},
			}, {
				// Open 802.11ac network on channel 36 with center channel of 42 and channel width of 80MHz.
				Name: "80211ac5vht80mixed",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211acMixed), hostapd.Channel(36), hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80), hostapd.VHTCenterChannel(42), hostapd.VHTChWidth(hostapd.VHTChWidth80)}},
					},
				},
			}, {
				// Open 802.11ac network on channel 157 with center channel of 155 and channel width of 80MHz.
				// The router is forced to use 80 MHz wide rates only.
				Name: "80211ac5vht80pure",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{[]hostapd.Option{hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(157), hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80), hostapd.VHTCenterChannel(155), hostapd.VHTChWidth(hostapd.VHTChWidth80)}},
					},
				},
			},
		},
	})
}

func SimpleConnect(ctx context.Context, s *testing.State) {
	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(ctx, s.DUT(), s.RPCHint(), router)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(ctx); err != nil {
			s.Logf("Failed to tear down test fixture, err=%q", err.Error())
		}
	}()

	testOnce := func(ctx context.Context, options []hostapd.Option) error {
		ap, err := tf.ConfigureAP(ctx, options...)
		if err != nil {
			return errors.Wrap(err, "failed to configure ap")
		}
		defer func() {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Logf("Failed to deconfig ap, err=%q", err.Error())
			}
		}()
		s.Log("AP setup done")

		if err := tf.ConnectWifi(ctx, ap); err != nil {
			return errors.Wrap(err, "failed to connect to WiFi")
		}
		defer func() {
			if err := tf.DisconnectWifi(ctx); err != nil {
				s.Logf("Failed to disconnect wifi, err=%q", err.Error())
			}
		}()
		s.Log("Connected")

		if err := tf.AssertNoDisconnect(ctx, tf.PingFromDUT); err != nil {
			return errors.Wrap(err, "failed to ping from DUT")
		}
		// TODO(crbug.com/1034875): Assert no deauth detected from the server side.
		// TODO(crbug.com/1034875): Maybe some more check on the WiFi capabilities to
		// verify we really have the settings as expected. (ref: crrev.com/c/1995105)
		s.Log("Deconfiguring")
		return nil
	}

	param := s.Param().(simpleConnectParam)
	for i, tc := range param.testcases {
		s.Logf("Testcase #%d", i)
		if err := testOnce(ctx, tc.apOptions); err != nil {
			s.Fatalf("testcase #%d failed with err=%s", i, err.Error())
		}
	}
	s.Log("Tearing down")
}
