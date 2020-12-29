// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"

	"chromiumos/tast/dut"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        AxMultiPeer,
		Desc:        "Tests our ability to connect multiple peers to our third-party ax routers",
		Contacts:    []string{"hinton@google.com", "chromeos-platform-connectivity@google.com"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Attr:        []string{"group:wificell"},
	})
}

func AxMultiPeer(ctx context.Context, s *testing.State) {
	peerCount := 5
	for i := 1; i < peerCount; i++ {
		peerHostname, _ := s.DUT().CompanionDeviceHostname(fmt.Sprintf("-wifipeer%d", i))
		wifiPeer, err := dut.New(peerHostname, s.DUT().KeyFile(), s.DUT().KeyDir(), nil)
		if err != nil {
			s.Error("Cannot create wifipeer DUT ", peerHostname)
		} else {
			err := wifiPeer.Connect(ctx)
			if err != nil {
				s.Log("Error establishing connection to peer ", peerHostname)
			}
			wifiutil.AxConnect(ctx, s, wifiPeer, "Velop-5G", "chromeos")
			wifiutil.AxConnect(ctx, s, s.DUT(), "Velop-5G", "chromeos")
			wifiutil.AxConnect(ctx, s, wifiPeer, "Rapture-5G-1", "chromeos")
			wifiutil.AxConnect(ctx, s, s.DUT(), "Rapture-5G-1", "chromeos")
			wifiutil.AxConnect(ctx, s, wifiPeer, "Juplink-RX4-1500", "chromeos")
			wifiutil.AxConnect(ctx, s, s.DUT(), "Juplink-RX4-1500", "chromeos")
			wifiutil.AxConnect(ctx, s, wifiPeer, "NETGEAR69-5G", "chromeos")
			wifiutil.AxConnect(ctx, s, s.DUT(), "NETGEAR69-5G", "chromeos")
		}
	}
}
