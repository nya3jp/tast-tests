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
		Func:         WifiCaps80211ac,
		Desc:         "Verifies DUT supports required 802.11ac capabilities",
		Contacts:     []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi"},
		HardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
	})
}

func WifiCaps80211ac(ctx context.Context, s *testing.State) {
	iwr := iw.NewLocalRunner()
	res, err := iwr.ListPhys(ctx)
	if err != nil {
		s.Fatal("ListPhys failed: ", err)
	}
	if len(res) == 0 {
		s.Fatal("Expect at least one wireless phy; found nothing")
	}
	if !res[0].SupportVHT {
		s.Error("Device doesn't support VHT")
	}
	if !res[0].SupportVHT80SGI {
		s.Error("Device doesn't support VHT80 short guard interval")
	}
}
