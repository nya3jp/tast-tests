// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"reflect"

	"chromiumos/tast/common/network/iw"
	localiw "chromiumos/tast/local/network/iw"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
	"chromiumos/tast/testing/wlan"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SAPCaps,
		Desc: "Verifies DUT supports SoftAP and a minimum set of required protocols",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func"},
		SoftwareDeps: []string{"wifi"},
		HardwareDeps: hwdep.D(hwdep.WifiSAP()),
	})
}

// SAPCaps verifies level of support for Soft AP according to the driver.
func SAPCaps(ctx context.Context, s *testing.State) {
	// This test checks that DUT has necessary capabilities to run Soft AP (MVP):
	// * Supports AP mode,
	// * 2.4GHz band,
	// * HE in AP mode if HE is present in STA mode.
	// Also, it detects opportunities for more functionalities:
	// * Max number of Clients (if information is present).
	// Concurrency is already checked by ConcurrencyCaps test, we won't do it here.
	dev, err := wlan.DeviceInfo()
	if err != nil {
		s.Fatal("Failed reading the WLAN device information: ", err)
	}
	// Log the actual device name.
	s.Log("Device: ", dev.Name)

	iwr := localiw.NewLocalRunner()
	res, out, err := iwr.ListPhys(ctx)
	if err != nil {
		s.Fatal("ListPhys failed: ", err)
	}
	if len(res) == 0 {
		s.Fatal("Expect at least one wireless phy; found nothing")
	}

	// Save `iw list` text to log file.
	ioutil.WriteFile(filepath.Join(s.OutDir(), "iw_list"), out, 0644)

	checkMode := func(phy *iw.Phy, mode iw.IfType) bool {
		for _, m := range phy.Modes {
			if m == string(mode) {
				testing.ContextLogf(ctx, "Device supports %s mode", mode)
				return true
			}
		}
		return false
	}

	checkBands := func(phy *iw.Phy) bool {
		var has24GHz bool
		for _, b := range phy.Bands {
			if len(b.FrequencyFlags) == 0 {
				// Just-in-case.
				continue
			}
			// Frequency is coded in map keys.
			keys := reflect.ValueOf(b.FrequencyFlags).MapKeys()
			f := keys[0].Interface().(int)
			if f >= 2412 && f <= 2484 {
				has24GHz = true
				// We're not looking for any more bands.
				break
			} else if f >= 5160 && f < 5925 {
				// 5GHz
			} else if f >= 5925 && f <= 7115 {
				// 6GHz
			} else {
				testing.ContextLogf(ctx, "Unknown frequency %d", f)
			}
		}
		// Minimum requirement for MVP
		return has24GHz
	}

	// We're using phys[0] basing on two assumptions:
	// 1. All phys of the same device will have the same capabilities.
	// 2. We support only one WiFi device per DUT.
	if !checkMode(res[0], iw.IfTypeAP) {
		s.Error("Device does not declare AP-mode support")
	}

	if !checkBands(res[0]) {
		s.Error("Device does not declare all expected bands (2.4G)")
	}

	if res[0].MaxSTAs > 0 {
		s.Log("Max concurrent clients: ", res[0].MaxSTAs)
		if res[0].MaxSTAs < 16 {
			s.Log("Less than 16 associated STAs are supported in AP mode: ", res[0].MaxSTAs)
		}
	}

	if res[0].SupportHESTA {
		if !res[0].SupportHEAP {
			s.Error("Device doesn't support HE in AP mode")
		}
	}

	if res[0].SupportHE40HE80STA {
		if !res[0].SupportHE40HE80AP {
			s.Error("Device doesn't support 5ghz HE in AP mode")
		}
	}

}
