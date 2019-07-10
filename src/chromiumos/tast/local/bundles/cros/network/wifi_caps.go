// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     WifiCaps,
		Desc:     "Verifies `iw` Timed Scan executes and is parsed properly",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

func WifiCaps(ctx context.Context, s *testing.State) {
	// Check if device is in station mode
	if res, err := iw.GetInterfaceAttributes(ctx, "wlan0"); err != nil {
		s.Fatal("GetInterfaceAttributes failed: ", err)
	} else if res.IfType != "managed" {
		s.Fatal("Interface is not in station (managed) mode")
	}

	res, err := iw.ListPhys(ctx)
	if err != nil {
		s.Fatal("ListPhys failed: ", err)
	}
	s.Logf("%+v\n", *(res[0]))
	// Check both 2.4 and 5 e9 bands supported
	supported24 := false
	supported5 := false
	for _, band := range res[0].Bands {
		for _, freq := range band.Frequencies {
			supported24 = supported24 || (freq >= 2400 && freq <= 2499)
			supported5 = supported5 || (freq >= 5000 && freq <= 5999)
		}
	}
	if !(supported24 && supported5) {
		s.Fatal("Device doesn't support both 2.4ghz and 5ghz bands")
	}
	// Check 11ac
	if !res[0].SupportVHT {
		s.Fatal("Device doesn't support 802.11ac")
	}
	// Check throughput support
	if !res[0].SupportHT2040 {
		s.Fatal("Device doesn't support all required throughput options: HT20, HT40, VHT80")
	}
	// Check short guard interval support
	if !res[0].SupportHT40SGI {
		s.Fatal("Device doesn't support HT40 compatible short guard interval")
	}

}
