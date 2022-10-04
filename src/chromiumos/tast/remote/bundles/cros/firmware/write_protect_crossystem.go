// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	fwUtils "chromiumos/tast/remote/bundles/cros/firmware/utils"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WriteProtectCrossystem,
		Desc:         "Verify that enabled and disabled hardware write protect is reflected in crossystem wpsw_cur",
		Contacts:     []string{"evanbenn@google.com", "cros-flashrom-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      5 * time.Minute,
		Fixture:      fixture.NormalMode,
	})
}

func WriteProtectCrossystem(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
		s.Fatal("Failed to disable WP: ", err)
	}

	if err := fwUtils.CheckCrossystemWPSW(ctx, h, 0); err != nil {
		s.Fatal("Failed to confirm WP is off: ", err)
	}

	if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOn); err != nil {
		s.Fatal("Failed to enable WP: ", err)
	}

	if err := fwUtils.CheckCrossystemWPSW(ctx, h, 1); err != nil {
		s.Fatal("Failed to confirm WP is on: ", err)
	}

	// Reset FWWP state to off before test end.
	if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
		s.Fatal("Failed to disable WP: ", err)
	}
}
