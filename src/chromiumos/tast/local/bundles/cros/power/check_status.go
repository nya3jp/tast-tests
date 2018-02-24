// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"os"
	"os/exec"
	"path/filepath"

	pow "chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckStatus,
		Desc: "Checks that dump_power_status can read power supply info from the kernel",
		Attr: []string{"bvt"},
	})
}

func CheckStatus(s *testing.State) {
	status, err := pow.GetStatus()
	if err != nil {
		s.Fatal("Failed to get power status: ", err)
	}
	if status.BatteryPresent {
		if status.BatteryPercent < 0.0 || status.BatteryPercent > 100.0 {
			s.Errorf("Battery percent %0.1f not in [0.0, 100.0]", status.BatteryPercent)
		}
		if status.BatteryDisplayPercent < 0.0 || status.BatteryDisplayPercent > 100.0 {
			s.Errorf("Battery display percent %0.1f not in [0.0, 100.0]", status.BatteryDisplayPercent)
		}
		// Strangely, the charge sometimes exceeds the full charge: https://crbug.com/815376
		if status.BatteryCharge > status.BatteryChargeFullDesign {
			s.Errorf("Battery charge %0.1f exceeds full design charge %0.1f",
				status.BatteryCharge, status.BatteryChargeFullDesign)
		}
		if status.BatteryChargeFull > status.BatteryChargeFullDesign {
			s.Errorf("Battery full charge %0.1f exceeds full design charge %0.1f",
				status.BatteryChargeFull, status.BatteryChargeFullDesign)
		}
	} else {
		if !status.LinePowerConnected {
			s.Error("No battery found, but line power also not present. Call the Nobel Committee!")
		}
	}

	// For good measure, also write the output of power_supply_info to a file.
	b, err := exec.Command("power_supply_info").CombinedOutput()
	if err != nil {
		s.Error("power_supply_info failed: ", err)
	}
	if f, err := os.Create(filepath.Join(s.OutDir(), "power_supply_info.txt")); err != nil {
		s.Error(err)
	} else {
		defer f.Close()
		if _, err := f.Write(b); err != nil {
			s.Error(err)
		}
	}
}
