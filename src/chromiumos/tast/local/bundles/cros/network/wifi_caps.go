// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/network/iw"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WifiCaps,
		Desc:         "Verifies DUT supports a minimum set of required protocols",
		Contacts:     []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi"},
	})
}

func WifiCaps(ctx context.Context, s *testing.State) {
	// Get WiFi interface:
	ifaces, err := iw.ListInterfaces(ctx)
	if err != nil {
		s.Fatal("ListInterfaces failed: ", err)
	}

	if len(ifaces) == 0 {
		s.Fatal("No wireless interfaces found")
	}

	res, err := iw.ListPhys(ctx)
	if err != nil {
		s.Fatal("ListPhys failed: ", err)
	}
	if len(res) == 0 {
		s.Fatal("ListPhys returned no device")
	}

	staSupported := false
	for _, mode := range res[0].Modes {
		if mode == "managed" {
			staSupported = true
		}
	}
	if !staSupported {
		s.Error("Station mode not supported")
	}

	// Check both 2.4 GHz and 5 GHz bands supported.
	supported24 := false
	supported5 := false
	for _, band := range res[0].Bands {
		for freq := range band.FrequencyFlags {
			if freq >= 2400 && freq <= 2499 {
				supported24 = true
			} else if freq >= 5000 && freq <= 5999 {
				supported5 = true
			}
		}
	}
	if !supported24 {
		s.Error("Device doesn't support 2.4ghz")
	}
	if !supported5 {
		s.Error("Device doesn't support 5ghz bands")
	}
	// TODO(crbug.com/1024554): Add back 802.11ac check after devices without it (e.g. monroe) reach their EOL.
	// Check throughput support.
	if !res[0].SupportHT2040 {
		s.Error("Device doesn't support all required throughput options: HT20, HT40, VHT80")
	}
	// Check short guard interval support.
	if !res[0].SupportHT40SGI {
		s.Error("Device doesn't support HT40 compatible short guard interval")
	}
}
