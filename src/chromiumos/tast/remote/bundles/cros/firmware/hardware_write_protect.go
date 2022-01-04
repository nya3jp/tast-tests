// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     HardwareWriteProtect,
		Desc:     "Checks that crossystem correctly reports HW WP state driven by servo",
		Contacts: []string{"nartemiev@google.com", "chromeos-firmware@google.com"},
		Attr:     []string{"group:firmware", "firmware_unstable"},
		VarDeps:  []string{"servo"},
		Fixture:  fixture.NormalMode,
	})
}

func HardwareWriteProtect(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	cases := []struct {
		fwwpState       servo.FWWPStateValue
		expectedWpswCur string
	}{
		{servo.FWWPStateOn, "1"},
		{servo.FWWPStateOff, "0"},
		{servo.FWWPStateOn, "1"},
		{servo.FWWPStateOff, "0"},
	}

	for _, c := range cases {
		if err := h.Servo.SetFWWPState(ctx, c.fwwpState); err != nil {
			s.Fatal("Failed to set servo FWWP state: ", err)
		}

		wpswCur, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamWpswCur)
		if err != nil {
			s.Fatal("Failed to get wpsw_cur value from crossystem: ", err)
		}
		s.Log("Current wpsw_cur value: ", wpswCur)

		if wpswCur != c.expectedWpswCur {
			s.Fatal("Incorrect wpsw_cur value returned by crossystem")
		}
	}
}
