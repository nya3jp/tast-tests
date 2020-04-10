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
		Func:         MTBF061WifiConnect,
		Desc:         "Supports 802.11g Wifi connection",
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline"},
		Contacts:     []string{"xliu@cienet.com"},
		Vars: []string{
			"wifi.802.11g.ssid",
			"wifi.802.11g.pwd",
			"dut.id",
			"detach.status.server",
			"allion.api.server",
			"allion.deviceId"},
	})
}

// MTBF061WifiConnect case verifies that the device can connect to a 802.11g router.
func MTBF061WifiConnect(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "mtbf061_test_wifi_connection_802.11g")
	caseName := "wifi.MTBF061WifiConnect"
	dutID := common.GetVar(ctx, s, "dut.id")
	detachStatusSvr := common.GetVar(ctx, s, "detach.status.server")
	allionServerURL := common.GetVar(ctx, s, "allion.api.server")
	deviceID := common.GetVar(ctx, s, "allion.deviceId")
	wifiSsid := common.GetVar(ctx, s, "wifi.802.11g.ssid")
	wifiPwd := common.GetVar(ctx, s, "wifi.802.11g.pwd")
	common.InformStatusServlet(ctx, s, detachStatusSvr, "start", dutID, caseName)
	defer common.InformStatusServlet(ctx, s, detachStatusSvr, "end", dutID, caseName)
	cr := s.PreValue().(*chrome.Chrome)
	defer st.End()
	s.Log("MTBF061WifiConnect - 802.11.g ssid: ", wifiSsid)
	wifiConn, mtbferr := wifi.NewConn(ctx, cr, true, wifiSsid, wifiPwd, allionServerURL, deviceID)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	defer wifiConn.Close(true)

	if mtbferr := wifiConn.TestConnected(); mtbferr != nil {
		s.Fatal(mtbferr)
	}
}
