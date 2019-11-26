// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SimpleConnect,
		Desc:         "PoC of SimpleConnect test using gRPC",
		Contacts:     []string{"yenlinlai@chromium.org"},
		Attr:         []string{"informational"}, // TODO: new group for wifi tests?
		SoftwareDeps: []string{},                // TODO: wificell dep?
		ServiceDeps:  []string{"tast.cros.network.Wifi"},
	})
}

func SimpleConnect(ctx context.Context, s *testing.State) {
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	wc := network.NewWifiClient(cl.Conn)

	config := &network.Config{
		Ssid: "GoogleGuest",
	}

	service, err := wc.Connect(ctx, config)
	if err != nil {
		s.Fatal("Failed to connect wifi: ", err)
	}
	_, err = wc.Disconnect(ctx, service)
	if err != nil {
		s.Fatal("Failed to disconnect: ", err)
	}
}
