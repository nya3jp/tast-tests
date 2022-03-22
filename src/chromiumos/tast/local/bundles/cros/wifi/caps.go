// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/local/network/iw"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/wlan"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Caps,
		Desc: "Verifies DUT supports a minimum set of required protocols",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:mainline", "group:wificell", "wificell_func"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
	})
}

func Caps(ctx context.Context, s *testing.State) {
	iwr := iw.NewLocalRunner()
	// Get the information of the WLAN device.
	dev, err := wlan.DeviceInfo()
	if err != nil {
		s.Fatal("Failed reading the WLAN device information: ", err)
	}

	res, err := iwr.ListPhys(ctx)
	if err != nil {
		s.Fatal("ListPhys failed: ", err)
	}
	if len(res) == 0 {
		s.Fatal("Expect at least one wireless phy; found nothing")
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
	if !res[0].SupportHT2040 {
		s.Error("Device doesn't support all required throughput options: HT20, HT40")
	}
	// Check short guard interval support.
	if !res[0].SupportHT20SGI {
		s.Error("Device doesn't support HT20 short guard interval")
	}
	if !res[0].SupportHT40SGI {
		s.Error("Device doesn't support HT40 short guard interval")
	}

	// Check MU-MIMO support. Older generations don't support MU-MIMO.
	// TODO(crbug.com/1024554): Move to WifiCaps80211ac after it is critical or merge
	// the two after monroe EOL.
	if dev.SupportMUMIMO() != res[0].SupportMUMIMO {
		if dev.SupportMUMIMO() {
			// New chips require MU-MIMO.
			s.Error("Device doesn't support MU-MIMO")
		} else {
			// We do not expect older chips to have MU-MIMO support.
			s.Error("Device unexpectedly supports MU-MIMO")
		}
	}
}
