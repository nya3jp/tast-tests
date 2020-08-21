// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF063WifiEnable,
		Desc:         "Enable/Disable WiFi network options",
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"xliu@cienet.com"},
	})
}

// MTBF063WifiEnable case verifies that Wifi network options can be disabled and then re-enabled.
func MTBF063WifiEnable(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "mtbf063_wifi_enable")
	cr := s.PreValue().(*chrome.Chrome)
	defer st.End()
	wifiConn, mtbferr := wifi.NewConn(ctx, cr, true, "", "", "", "")

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	defer wifiConn.Close(true)
	wifiConn.EnterWifiPage()

	if wifiStatus, mtbferr := wifiConn.DisableWifi(); mtbferr != nil {
		s.Fatal(mtbferr)
	} else if wifiStatus != "Off" {
		s.Fatal(mtbferrors.New(mtbferrors.WIFIEnableF, nil))
	}

	if wifiListDisplayed, mtbferr := wifiConn.CheckWifiListDisplayed(); mtbferr != nil {
		s.Fatal(mtbferr)
	} else if wifiListDisplayed {
		s.Fatal(mtbferrors.New(mtbferrors.WIFIDisableF, nil))

	}

	if wifiStatus, mtbferr := wifiConn.EnableWifi(); mtbferr != nil {
		s.Fatal(mtbferr)
	} else if wifiStatus == "Off" {
		s.Fatal(mtbferrors.New(mtbferrors.WIFIEnableF, nil))
	}

	if wifiListDisplayed, mtbferr := wifiConn.CheckWifiListDisplayed(); mtbferr != nil {
		s.Fatal(mtbferr)
	} else if !wifiListDisplayed {
		s.Fatal(mtbferrors.New(mtbferrors.WIFIEnableF, nil))
	}
}
