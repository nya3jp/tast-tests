// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/wifi/common"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	wifi80211gSsid = "wifi.802.11g.ssid"
	wifi80211gPwd  = "wifi.802.11g.pwd"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF061WifiConnect,
		Desc:         "Supports 802.11g Wifi connection",
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline"},
		Contacts:     []string{"xliu@cienet.com"},
		Vars:         []string{"wifi.802.11g.ssid", "wifi.802.11g.pwd", "dut.id", "detach.status.server"},
	})
}

// MTBF061WifiConnect case verifies that the device can connect to a 802.11g router.
func MTBF061WifiConnect(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "mtbf061_test_wifi_connection_802.11g")
	dutID := common.GetVar(ctx, s, "dut.id")
	detachStatusSvr := common.GetVar(ctx, s, "detach.status.server")
	common.InformStatusServlet(ctx, s, detachStatusSvr, "start", dutID)
	defer common.InformStatusServlet(ctx, s, detachStatusSvr, "end", dutID)

	cr := s.PreValue().(*chrome.Chrome)
	defer st.End()

	wifiSsid, ok := s.Var(wifi80211gSsid)

	if !ok {
		s.Fatal("MTBF failed: ", mtbferrors.New(mtbferrors.OSVarRead, nil, wifi80211gSsid))
	}

	wifiPwd, ok := s.Var(wifi80211gPwd)

	if !ok {
		s.Fatal("MTBF failed: ", mtbferrors.New(mtbferrors.OSVarRead, nil, wifi80211gPwd))
	}

	s.Log("MTBF061WifiConnect - 802.11.g ssid: ", wifiSsid)
	wifiConn, err := wifi.NewConn(ctx, cr, true, wifiSsid, wifiPwd)

	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	defer wifiConn.Close()
	if err := wifiConn.TestConnected(); err != nil {
		s.Fatal("MTBF failed: ", err)
	}
}
