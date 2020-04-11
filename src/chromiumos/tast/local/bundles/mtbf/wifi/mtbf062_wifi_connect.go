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
	wifi80211nSsid = "wifi.802.11n.ssid"
	wifi80211nPwd  = "wifi.802.11n.pwd"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF062WifiConnect,
		Desc:         "Supports 802.11n MIMO (multiple input multiple output) router for Wifi connection",
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline"},
		Contacts:     []string{"xliu@cienet.com"},
		Vars:         []string{"wifi.802.11n.ssid", "wifi.802.11n.pwd", "dut.id", "detach.status.server"},
	})
}

// MTBF062WifiConnect case verifies that the device can connect to a 802.11n router
func MTBF062WifiConnect(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "mtbf062_test_wifi_connection_802.11n")
	dutID := common.GetVar(ctx, s, "dut.id")
	detachStatusSvr := common.GetVar(ctx, s, "detach.status.server")
	common.InformStatusServlet(ctx, s, detachStatusSvr, "start", dutID)
	defer common.InformStatusServlet(ctx, s, detachStatusSvr, "end", dutID)

	cr := s.PreValue().(*chrome.Chrome)
	defer st.End()
	wifiSsid, ok := s.Var(wifi80211nSsid)

	if !ok {
		s.Fatal("MTBF failed: ", mtbferrors.New(mtbferrors.OSVarRead, nil, wifi80211nSsid))
	}

	wifiPwd, ok := s.Var(wifi80211nPwd)

	if !ok {
		s.Fatal("MTBF failed: ", mtbferrors.New(mtbferrors.OSVarRead, nil, wifi80211nPwd))
	}

	s.Log("MTBF062WifiConnect - 802.11.n ssid: ", wifiSsid)

	wifiConn, err := wifi.NewConn(ctx, cr, true, wifiSsid, wifiPwd)

	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	defer wifiConn.Close()
	wifiConn.TestConnected()
}
