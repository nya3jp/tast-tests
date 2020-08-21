// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"strconv"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF060VerifyWifi,
		Desc:         "MTBF060 subcase to verify if WiFi is enabled or disabled",
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"xliu@cienet.com"},
		Params: []testing.Param{{
			Name: "enabled",
			Val:  []string{"true"},
		}, {
			Name: "disabled",
			Val:  []string{"false"},
		}},
	})
}

// MTBF060VerifyWifi subcase of MTBF060 to verify if WiFi is enabled or disabled.
func MTBF060VerifyWifi(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "mtbf060_verify_wifi")
	cr := s.PreValue().(*chrome.Chrome)
	defer st.End()
	vars := s.Param().([]string)
	enabled := vars[0]

	s.Log("MTBF060VerifyWifi - enabled: ", enabled)
	shouldEnable, err := strconv.ParseBool(enabled)

	if err != nil {
		shouldEnable = false
	}

	wifiConn, mtbferr := wifi.NewConn(ctx, cr, false, "", "", "", "")

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	defer wifiConn.Close(false)
	wifiStatus, mtbferr := wifiConn.CheckWifi(shouldEnable)
	s.Log("MTBF060VerifyWifi - wifiStatus: ", wifiStatus)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
}
