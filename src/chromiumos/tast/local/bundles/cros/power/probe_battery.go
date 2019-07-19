// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	pow "chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeBattery,
		Desc: "Checks that device is on battery and not on AC",
		Contacts: []string{
			"chromeos-power@google.com",
			"mqg@chromium.org",
		},
		Attr: []string{"informational"},
	})
}

func ProbeBattery(ctx context.Context, s *testing.State) {
	status, err := pow.GetStatus(ctx)
	if err != nil {
		s.Fatal("Failed to get power status: ", err)
	}
	if !status.BatteryPresent {
		s.Fatal("No battery present")
	}
	if !(status.BatteryStatus == "Discharging" && status.BatteryDischarging) {
		s.Fatal("No battery discharging")
	}
	if status.LinePowerConnected {
		s.Fatal("Line power is connected")
	}
}
