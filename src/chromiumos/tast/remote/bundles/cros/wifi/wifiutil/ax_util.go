// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

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

// AxConnect connects the specified DUT to the AX router.
func AxConnect(ctx context.Context, s *testing.State, dut *dut.DUT, ssid, passphrase string) {
	defer func(ctx context.Context) {
		rpc, _ := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
		wificlient := network.NewWifiServiceClient(rpc.Conn)
		disconnectWifi(ctx, wificlient, false)
	}(ctx)
	s.Log("Connecting to Ax Router ", ssid)
	rpc, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
	if err != nil {
		s.Error("Error trying to create RPC Dial: ", err)
	}
	wificlient := network.NewWifiServiceClient(rpc.Conn)
	secConf := &wpa.Config{}
	secProps, err := secConf.ShillServiceProperties()
	if err != nil {
		s.Error("Error Creating Security Properties: ", err)
	}
	secProps["Passphrase"] = "chromeos"
	propsEnc, err := protoutil.EncodeToShillValMap(secProps)
	if err != nil {
		s.Error("Error encoding Security Properties to ShillValMaps: ", err)
	}

	err = disconnectWifi(ctx, wificlient, false)
	if err == nil {
		s.Log("Previous test did not clean up, Wifi was already connected. Disconnected it")
	}

	request := &network.ConnectRequest{
		Ssid:       []byte("NETGEAR69"),
		Hidden:     false,
		Security:   secConf.Class(),
		Shillprops: propsEnc,
	}

	response, err := wificlient.Connect(ctx, request)
	if err != nil {
		s.Error("Error connecting DUT to AP: ", err)
	}
	s.Log("Connection establish. Response was ", response)
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
