// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/wifiservice"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ConnectWifi,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify Wi-Fi connectivity",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.wifiservice.WifiService"},
		Vars:         []string{"wifissid", "wifipassword"},
	})
}

func ConnectWifi(ctx context.Context, s *testing.State) {
	d := s.DUT()
	ssid := s.RequiredVar("wifissid")
	password := s.RequiredVar("wifipassword")

	wifiReq := &wifiservice.WifiRequest{Ssid: ssid, Password: password}

	// Login to chrome.
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	cr := wifiservice.NewWifiServiceClient(cl.Conn)
	if _, err := cr.New(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	s.Log("Connecting WiFi")
	if _, err := cr.ConnectWifi(ctx, wifiReq); err != nil {
		s.Fatal("Failed to connect to Wi-Fi: ", err)
	}

	s.Log("Disconnecting ethernet")
	if _, err := cr.DownEth(ctx, &epty.Empty{}); err != nil {
		s.Fatal("Failed to disconnect ethernet: ", err)
	}
	s.Log("Connecting back to ethernet")
	if _, err := cr.UpEth(ctx, &empty.Empy{}); err != nil {
		s.Fatal("Failed to connect ethernet: ", err)
	}
}
