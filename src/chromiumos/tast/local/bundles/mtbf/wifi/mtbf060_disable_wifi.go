// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF060DisableWifi,
		Desc:         "MTBF060DisableWifi is a subcase of MTBF060 for disabling WiFi",
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"xliu@cienet.com"},
	})
}

// MTBF060DisableWifi is a subcase of MTBF060 for disabling WiFi
func MTBF060DisableWifi(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "mtbf060_disable_wifi")
	cr := s.PreValue().(*chrome.Chrome)
	defer st.End()

	wifiConn, mtbferr := wifi.NewConn(ctx, cr, false, "", "", "", "")

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	defer wifiConn.Close(true)
	wifiConn.DisableWifi()
}
