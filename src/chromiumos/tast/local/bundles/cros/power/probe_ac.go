// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeAC,
		Desc: "Checks that device is on AC",
		Contacts: []string{
			"chromeos-power@google.com",
			"mqg@chromium.org",
		},
		Attr: []string{"informational"},
	})
}

func ProbeAC(ctx context.Context, s *testing.State) {
	status, err := power.GetStatus(ctx)
	if err != nil {
		s.Fatal("Failed to get power status: ", err)
	}
	if !status.LinePowerConnected {
		s.Fatal("Line power is not connected")
	}
	if !status.BatteryPresent {
		s.Log("No battery present, might be Chromebox")
		return
	}
	if status.BatteryStatus == "Charging" && !status.BatteryDischarging {
		s.Log("Battery is charging")
		return
	}
	if status.BatteryPercent > 95 {
		s.Log("DUT battery discharging but deemed ok")
	}
	s.Fatal("Battery is discharging")
}
