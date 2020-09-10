// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/network/iw"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WifiHeCaps,
		Desc:         "Verifies DUT supports a minimum set of required protocols",
		Contacts:     []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:         []string{"group:informational"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
		HardwareDeps: hwdep.D(hwdep.Platform("hatch")),
	})
}

func WifiHeCaps(ctx context.Context, s *testing.State) {
	iwr := iw.NewLocalRunner()

	// Get the information of the WLAN device.
	res, err := iwr.ListPhys(ctx)
	if err != nil {
		s.Fatal("ListPhys failed: ", err)
	}
	if len(res) == 0 {
		s.Fatal("Expect at least one wireless phy; found nothing")
	}
	if !res[0].SupportHE {
		s.Error("Device doesn't support HE capabilities")
	}
	if !res[0].SupportHE160 {
		s.Error("Device doesn't support 5ghz HE capabilities")
	}

}
