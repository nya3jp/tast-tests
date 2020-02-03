// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
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
			s.Log("Failed to tear down test fixture, err: ", err)
		}
	}()

	testOnce := func(ctx context.Context, options []hostapd.Option) {
		ap, err := tf.ConfigureAP(ctx, options...)
		if err != nil {
			s.Error("Failed to configure ap, err: ", err)
			return
		}
		defer func() {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig ap, err: ", err)
			}
		}()
		s.Log("AP setup done")

		if err := tf.ConnectWifi(ctx, ap); err != nil {
			s.Error("Failed to connect to WiFi, err: ", err)
			return
		}
		defer func() {
			if err := tf.DisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi, err: ", err)
			}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.SSID{Ssid: ap.Config().Ssid}); err != nil {
				s.Errorf("Failed to remove entries for ssid=%s, err: %s", ap.Config().Ssid, err.Error())
			}
		}()
		s.Log("Connected")

		ping := func(ctx context.Context) error {
			return tf.PingFromDUT(ctx)
		}

		if err := tf.AssertNoDisconnect(ctx, ping); err != nil {
			s.Error("Failed to ping from DUT, err: ", err)
			return
		}
		// TODO(crbug.com/1034875): Assert no deauth detected from the server side.
		// TODO(crbug.com/1034875): Maybe some more check on the WiFi capabilities to
		// verify we really have the settings as expected. (ref: crrev.com/c/1995105)
		s.Log("Deconfiguring")
		return
	}

	param := s.Param().(simpleConnectParam)
	for i, tc := range param.testcases {
		s.Logf("Testcase #%d", i)
		testOnce(ctx, tc.apOptions)
		if s.HasError() {
			// The testcase failed, let's fatal here.
			s.Fatalf("Testcase #%d failed", i)
		}
	}
	s.Log("Tearing down")
}
