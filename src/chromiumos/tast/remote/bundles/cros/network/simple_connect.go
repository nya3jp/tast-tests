// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wifi"
	"chromiumos/tast/remote/wifi/hostap"
	"chromiumos/tast/testing"
)

type simpleConnectTestcase struct {
	apOptions []hostap.Option
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
						{[]hostap.Option{hostap.Mode(hostap.Mode80211a), hostap.Channel(48)}},
						{[]hostap.Option{hostap.Mode(hostap.Mode80211a), hostap.Channel(64)}},
					},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11b network on channels 1, 6, 11.
				Name: "80211b",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{[]hostap.Option{hostap.Mode(hostap.Mode80211b), hostap.Channel(1)}},
						{[]hostap.Option{hostap.Mode(hostap.Mode80211b), hostap.Channel(6)}},
						{[]hostap.Option{hostap.Mode(hostap.Mode80211b), hostap.Channel(11)}},
					},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11g network on channels 1, 6, 11.
				Name: "80211g",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{[]hostap.Option{hostap.Mode(hostap.Mode80211g), hostap.Channel(1)}},
						{[]hostap.Option{hostap.Mode(hostap.Mode80211g), hostap.Channel(6)}},
						{[]hostap.Option{hostap.Mode(hostap.Mode80211g), hostap.Channel(11)}},
					},
				},
			},
		},
	})
}

func SimpleConnect(ctx context.Context, s *testing.State) {
	router, _ := s.Var("router")
	tf, err := wifi.NewTestFixture(ctx, s.DUT(), s.RPCHint(), router)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(ctx); err != nil {
			s.Logf("Failed to tear down test fixture, err=%q", err.Error())
		}
	}()

	testOnce := func(ctx context.Context, options []hostap.Option) error {
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
			return errors.Wrap(err, "failed to connect to wifi")
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
		// TODO(yenlinlai): assert no deauth detected from the server side.
		// TODO(yenlinlai): Maybe some more check on the wifi capabilities to verify
		// we really have the settings as expected. (ref: crrev.com/c/1995105)
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
