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
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"xliu@cienet.com"},
		Vars: []string{
			"wifi.80211gSsid",
			"wifi.80211gPwd",
			"wifi.dutId",
			"wifi.detachStatusServer",
			"wifi.allionApiServer",
			"wifi.allionDevId"},
	})
}

// MTBF061WifiConnect case verifies that the device can connect to a 802.11g router.
func MTBF061WifiConnect(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "mtbf061_test_wifi_connection_802.11g")
	caseName := "wifi.MTBF061WifiConnect"
	dutID := s.RequiredVar("wifi.dutId")
	detachStatusSvr := s.RequiredVar("wifi.detachStatusServer")
	allionServerURL := s.RequiredVar("wifi.allionApiServer")
	deviceID := s.RequiredVar("wifi.allionDevId")
	wifiSsid := s.RequiredVar("wifi.80211gSsid")
	wifiPwd := s.RequiredVar("wifi.80211gPwd")
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
