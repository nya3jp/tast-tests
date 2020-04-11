// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/local/bundles/mtbf/wifi/common"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF062WifiConnect,
		Desc:         "Supports 802.11n MIMO (multiple input multiple output) router for Wifi connection",
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline"},
		Contacts:     []string{"xliu@cienet.com"},
		Vars: []string{
			"wifi.802.11n.ssid",
			"wifi.802.11n.pwd",
			"dut.id",
			"detach.status.server",
			"allion.api.server",
			"allion.deviceId"},
	})
}

// MTBF062WifiConnect case verifies that the device can connect to a 802.11n router
func MTBF062WifiConnect(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "mtbf062_test_wifi_connection_802.11n")
	caseName := "wifi.MTBF062WifiConnect"
	dutID := common.GetVar(ctx, s, "dut.id")
	detachStatusSvr := common.GetVar(ctx, s, "detach.status.server")
	allionServerURL := common.GetVar(ctx, s, "allion.api.server")
	deviceID := common.GetVar(ctx, s, "allion.deviceId")
	wifiSsid := common.GetVar(ctx, s, "wifi.802.11n.ssid")
	wifiPwd := common.GetVar(ctx, s, "wifi.802.11n.pwd")
	s.Logf("MTBF062WifiConnect - allionServerURL=%v, deviceID=%v ssid=%v", allionServerURL, deviceID, wifiSsid)
	common.InformStatusServlet(ctx, s, detachStatusSvr, "start", dutID, caseName)
	defer common.InformStatusServlet(ctx, s, detachStatusSvr, "end", dutID, caseName)
	cr := s.PreValue().(*chrome.Chrome)
	defer st.End()
	s.Log("MTBF062WifiConnect - 802.11.n ssid: ", wifiSsid)
	wifiConn, mtbferr := wifi.NewConn(ctx, cr, true, wifiSsid, wifiPwd, allionServerURL, deviceID)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	defer wifiConn.Close(true)

	if mtbferr := wifiConn.TestConnected(); mtbferr != nil {
		s.Fatal(mtbferr)
	}
}
