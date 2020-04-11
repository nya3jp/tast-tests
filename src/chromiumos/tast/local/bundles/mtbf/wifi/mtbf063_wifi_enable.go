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
		Attr:         []string{"group:mainline"},
		Contacts:     []string{"xliu@cienet.com"},
	})
}

// MTBF063WifiEnable case verifies that Wifi network options can be disabled and then re-enabled.
func MTBF063WifiEnable(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "mtbf063_wifi_enable")
	cr := s.PreValue().(*chrome.Chrome)
	defer st.End()
	wifiConn, err := wifi.NewConn(ctx, cr, true, "", "")

	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	defer wifiConn.Close()
	wifiConn.EnterWifiPage()

	if wifiStatus, err := wifiConn.DisableWifi(); err != nil {
		s.Fatal("MTBF failed: ", err)
	} else if wifiStatus != "Off" {
		s.Fatal("MTBF failed: ", mtbferrors.New(mtbferrors.WIFIEnableF, nil))
	}

	if wifiListDisplayed, err := wifiConn.CheckWifiListDisplayed(); err != nil {
		s.Fatal("MTBF failed: ", err)
	} else if wifiListDisplayed {
		s.Fatal("MTBF failed: ", mtbferrors.New(mtbferrors.WIFIDisableF, nil))

	}

	if wifiStatus, err := wifiConn.EnableWifi(); err != nil {
		s.Fatal("MTBF failed: ", err)
	} else if wifiStatus == "Off" {
		s.Fatal("MTBF failed: ", mtbferrors.New(mtbferrors.WIFIEnableF, nil))
	}

	if wifiListDisplayed, err := wifiConn.CheckWifiListDisplayed(); err != nil {
		s.Fatal("MTBF failed: ", err)
	} else if !wifiListDisplayed {
		s.Fatal("MTBF failed: ", mtbferrors.New(mtbferrors.WIFIEnableF, nil))
	}
}
