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
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

type parm struct {
	apOptions []wifi.HostAPOption
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SimpleConnect,
		Desc:         "PoC of SimpleConnect test using gRPC",
		Contacts:     []string{"yenlinlai@google.com"},
		Attr:         []string{"informational"}, // TODO: new group for wifi tests?
		SoftwareDeps: []string{},                // TODO: wificell dep?
		ServiceDeps:  []string{"tast.cros.network.Wifi"},
		Vars:         []string{"router"},
		Params: []testing.Param{{
			Name: "poc",
			Val: parm{
				apOptions: nil,
			},
		}},
	})
}

func SimpleConnect(ctx context.Context, s *testing.State) {
	// Seed the random to avoid ssid collision.
	rand.Seed(time.Now().UnixNano())

	dut := s.DUT()
	parm := s.Param().(parm)

	// For now, router's SSH key is the same as DUT's.
	// TODO: still keep this assumption?
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
	ssid := wifi.RandomSSID("TAST_TEST_")
	apConf := wifi.NewHostAPConfig(ssid, parm.apOptions...)

	iface, err := router.SelectInterface(ctx, apConf)
	if err != nil {
		s.Fatal("Cannot get a wireless interface from the AP router: ", err)
	}
	hostap, err := wifi.NewHostAPServer(ctx, router, iface, apConf)
	if err != nil {
		s.Fatal("Failed to create host AP server: ", err)
	}
	defer func() {
		if err := hostap.Stop(ctx); err != nil {
			s.Log("Failed to stop host AP server: ", err)
		}
	}()

	dhcpConf := wifi.NewDHCPConfig(0)
	dhcp, err := wifi.NewDHCPServer(ctx, router, iface, dhcpConf)
	if err != nil {
		s.Fatal("Failed to create dhcp server: ", err)
	}
	defer func() {
		if err := dhcp.Stop(ctx); err != nil {
			s.Log("Failed to stop dhcp: ", err)
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
		Ssid: ssid,
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
		_, err = wc.DeleteEntriesForSSID(ctx, &network.SSID{Ssid: ssid})
		if err != nil {
			s.Log("Failed to delete profile entries: ", err)
		}
	}()

	s.Log("Connected")

	// Ping from dut to router.
	pr := ping.NewRunner(dut)
	res, err := pr.Ping(ctx, dhcp.ServerIP().String())
	if err != nil {
		s.Fatal("Failed to ping dhcp server: ", err)
	}
	s.Logf("ping statistics=%+v", res)

	if res.Sent != res.Received {
		s.Fatal("Some packets are lost in ping")
	}

	s.Log("Tearing down")
}
