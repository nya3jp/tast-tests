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
		Func:         MTBF068WifiConnect,
		Desc:         "MTBF068 to verify device can connect to 802.11ac (Wave2) router on 5ghz channel",
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"xliu@cienet.com"},
		Vars: []string{
			"wifi.80211acWave2Ssid",
			"wifi.80211acWave2Pwd",
			"wifi.dutId",
			"wifi.detachStatusServer",
			"wifi.allionApiServer",
			"wifi.allionDevId"},
	})
}

// MTBF068WifiConnect testing the test case TC61
func MTBF068WifiConnect(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "mtbf068_test_wifi_connection_802.11ac_wave1")
	caseName := "wifi.MTBF068WifiConnect"
	dutID := s.RequiredVar("wifi.dutId")
	detachStatusSvr := s.RequiredVar("wifi.detachStatusServer")
	allionServerURL := s.RequiredVar("wifi.allionApiServer")
	deviceID := s.RequiredVar("wifi.allionDevId")
	wifiSsid := s.RequiredVar("wifi.80211acWave2Ssid")
	wifiPwd := s.RequiredVar("wifi.80211acWave2Pwd")
	common.InformStatusServlet(ctx, s, detachStatusSvr, "start", dutID, caseName)
	defer common.InformStatusServlet(ctx, s, detachStatusSvr, "end", dutID, caseName)
	cr := s.PreValue().(*chrome.Chrome)
	defer st.End()
	wifiConn, mtbferr := wifi.NewConn(ctx, cr, true, wifiSsid, wifiPwd, allionServerURL, deviceID)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	defer wifiConn.Close(true)

	if mtbferr := wifiConn.TestConnected(); mtbferr != nil {
		s.Fatal(mtbferr)
	}
}
