// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/network/protoutil"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        AxBasicConnect,
		Desc:        "Tests our ability to connect to our third-party ax routers",
		Contacts:    []string{"hinton@google.com", "chromeos-wifi-champs@google.com"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Attr:        []string{"group:wificell"},
	})
}

func AxBasicConnect(ctx context.Context, s *testing.State) {
	for _, tc := range []struct {
		ssid     string
		password string
		serverIP string
	}{
		{
			ssid:     "Velop-5G",
			password: "chromeos",
			serverIP: "192.168.1.1",
		},
		{
			ssid:     "Rapture-5G-1",
			password: "chromeos",
			serverIP: "192.168.50.1",
		},
		{
			ssid:     "Juplink-RX4-1500",
			password: "chromeos",
			serverIP: "192.168.0.1",
		},
		{
			ssid:     "NETGEAR69-5G",
			password: "chromeos",
			serverIP: "192.168.1.1",
		},
	} {
		s.Run(ctx, tc.ssid, func(ctx context.Context, s *testing.State) {
			axConnect(ctx, s.DUT(), s.RPCHint(), tc.ssid, tc.password, tc.serverIP)
		})
	}
}

func axConnect(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint, ssid, passphrase, serverIP string) {
	defer func(ctx context.Context) {
		rpc, _ := rpc.Dial(ctx, dut, rpcHint, "cros")
		wificlient := network.NewWifiServiceClient(rpc.Conn)
		disconnectWifi(ctx, wificlient, false)
	}(ctx)
	rpc, err := rpc.Dial(ctx, dut, rpcHint, "cros")
	if err != nil {
		errors.Wrap(err, "error trying to create RPC Dial")
	}
	wificlient := network.NewWifiServiceClient(rpc.Conn)
	secConf := &wpa.Config{}
	secProps, err := secConf.ShillServiceProperties()
	if err != nil {
		errors.Wrap(err, "error Creating Security Properties")
	}
	secProps["Passphrase"] = "chromeos"
	propsEnc, err := protoutil.EncodeToShillValMap(secProps)
	if err != nil {
		errors.Wrap(err, "error encoding Security Properties to ShillValMaps")
	}

	// Check to see if the previous test left us connected to a router.
	err = disconnectWifi(ctx, wificlient, false)
	// if err is nothing, then we were connected, which is bad, but we cleaned it up by disconnecting.
	if err == nil {
		testing.ContextLog(ctx, "Previous test did not clean up, Wifi was already connected. Disconnected it")
	}

	testing.ContextLogf(ctx, "Connecting to Ax Router %s", ssid)
	request := &network.ConnectRequest{
		Ssid:       []byte(ssid),
		Hidden:     false,
		Security:   secConf.Class(),
		Shillprops: propsEnc,
	}

	response, err := wificlient.Connect(ctx, request)
	if err != nil {
		errors.Wrap(err, "error connecting DUT to AP")
	}
	testing.ContextLogf(ctx, "Connection established. Response was %s", response)
	disconnectWifi(ctx, wificlient, false)
	// TODO(hinton): Add remote ping test here. Likely just ping the ssid's IP.
}

func disconnectWifi(ctx context.Context, wificlient network.WifiServiceClient, removeProfile bool) error {
	resp, err := wificlient.SelectedService(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "failed to get selected service")
	}

	req := &network.DisconnectRequest{
		ServicePath:   resp.ServicePath,
		RemoveProfile: removeProfile,
	}
	if _, err := wificlient.Disconnect(ctx, req); err != nil {
		return errors.Wrap(err, "failed to disconnect")
	}
	return nil
}
