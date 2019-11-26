// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"math/rand"
	"time"

	"chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wifi"
	"chromiumos/tast/remote/wifi/hostap"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
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
	// Seed the random to avoid ssid collision.
	rand.Seed(time.Now().UnixNano())

	dut := s.DUT()
	parm := s.Param().(simpleConnectParm)

	// For now, router's SSH key is the same as DUT's.
	router, err := wifi.NewRouter(ctx, s.RequiredVar("router"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to create router object: ", err)
	}
	defer func() {
		if err := router.Close(ctx); err != nil {
			s.Log("Failed to stop router")
		}
	}()

	// Set up AP.
	apConf := hostap.NewConfig(parm.apOptions...)
	hostap, err := router.StartHostAP(ctx, apConf)
	if err != nil {
		s.Fatal("Failed to setup hostap: ", err)
	}
	defer func() {
		if err := router.StopHostAP(ctx, hostap); err != nil {
			s.Log("Failed to stop hostap: ", err)
		}
	}()

	s.Log("AP setup done, try to connect")

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	wc := network.NewWifiClient(cl.Conn)

	config := &network.Config{
		Ssid: apConf.Ssid,
	}
	service, err := wc.Connect(ctx, config)
	if err != nil {
		s.Fatal("Failed to connect wifi: ", err)
	}
	defer func() {
		_, err = wc.Disconnect(ctx, service)
		if err != nil {
			s.Log("Failed to disconnect: ", err)
		}
		_, err = wc.DeleteEntriesForSSID(ctx, &network.SSID{Ssid: apConf.Ssid})
		if err != nil {
			s.Log("Failed to delete profile entries: ", err)
		}
	}()

	s.Log("Connected")

	// Ping from dut to router.
	pr := ping.NewRunner(dut)
	res, err := pr.Ping(ctx, hostap.ServerIP().String())
	if err != nil {
		s.Fatal("Failed to ping dhcp server: ", err)
	}
	s.Logf("ping statistics=%+v", res)

	if res.Sent != res.Received {
		s.Fatal("Some packets are lost in ping")
	}

	s.Log("Tearing down")
}
