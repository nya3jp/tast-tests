// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"io/ioutil"
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
	} else {
		// powerd should automatically report that line power is connected if there's no battery
		// regardless of what sysfs says.
		if !status.LinePowerConnected {
			s.Error("No battery found, but line power also not present")
		}
	}

	// For good measure, also write the output of power_supply_info to a file.
	b, err := exec.Command("power_supply_info").CombinedOutput()
	if err != nil {
		s.Error("power_supply_info failed: ", err)
	}
	if err = ioutil.WriteFile(filepath.Join(s.OutDir(), "power_supply_info.txt"), b, 0644); err != nil {
		s.Error("Writing output file failed: ", err)
	}
}
