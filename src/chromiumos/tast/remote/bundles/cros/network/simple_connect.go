// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/remote/wifi"
	"chromiumos/tast/remote/wifi/hostap"
	"chromiumos/tast/testing"
)

type simpleConnectParm struct {
	apOptions []hostap.Option
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        SimpleConnect,
		Desc:        "PoC of Wifi SimpleConnect test using gRPC",
		Contacts:    []string{"yenlinlai@google.com"}, // TODO(crbug.com/1034878): add wifi group here.
		Attr:        []string{"informational"},
		ServiceDeps: []string{"tast.cros.network.Wifi"},
		Vars:        []string{"router"},
		Params: []testing.Param{
			{
				Name: "poc",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211nPure),
						hostap.Channel(6),
						hostap.HTCaps(hostap.HTCapHT40Plus),
					},
				},
			}, {
				Name: "80211a",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211a),
						hostap.Channel(48),
					},
				},
			}, {
				Name: "80211b",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211b),
						hostap.Channel(1),
					},
				},
			}, {
				Name: "80211g",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
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
	parm := s.Param().(simpleConnectParm)
	ap, err := tf.ConfigureAP(ctx, parm.apOptions...)
	if err != nil {
		s.Fatal("Failed to configure ap: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Logf("Failed to deconfig ap, err=%q", err.Error())
		}
	}()
	s.Log("AP setup done")

	if err := tf.ConnectWifi(ctx, ap); err != nil {
		s.Fatal("Failed to connect to wifi: ", err)
	}
	defer func() {
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Logf("Failed to disconnect wifi, err=%q", err.Error())
		}
	}()
	s.Log("Connected")

	if err := tf.PingFromDUT(ctx); err != nil {
		s.Fatal("Failed to ping from DUT: ", err)
	}
	s.Log("Tearing down")
}
