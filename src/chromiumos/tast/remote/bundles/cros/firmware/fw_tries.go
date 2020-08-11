// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FWTries,
		Desc:         "Verify that the DUT can be specified to boot from A or B",
		Contacts:     []string{"cros-fw-engprod@google.com"},
		SoftwareDeps: []string{"crossystem"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func FWTries(ctx context.Context, s *testing.State) {
	d := s.DUT()
	vboot2, err := firmware.Vboot2(ctx, d)
	if err != nil {
		s.Fatal("Failed to determine fw_vboot2: ", err)
	}
	if vboot2 {
		s.Log("DUT uses vboot2")
	} else {
		s.Log("DUT does not use vboot2")
	}

	if err := firmware.CheckFWTries(ctx, d, firmware.A, firmware.UnspecifiedRWSection, 0); err != nil {
		s.Error("Error with DUT state at start of test: ", err)
	}
	if err := firmware.SetFWTries(ctx, d, firmware.B, 2); err != nil {
		s.Fatal("Setting FWTries to B/2: ", err)
	}
	if err := firmware.CheckFWTries(ctx, d, firmware.A, firmware.B, 2); err != nil {
		s.Error("After setting FWTries to B/2, before rebooting: ", err)
	}

	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Rebooting: ", err)
	}
	if err := firmware.CheckFWTries(ctx, d, firmware.B, firmware.B, 1); err != nil {
		s.Error("After setting FWTries to B/2, then rebooting once: ", err)
	}

	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Rebooting: ", err)
	}
	if err := firmware.CheckFWTries(ctx, d, firmware.B, firmware.UnspecifiedRWSection, 0); err != nil {
		s.Error("After setting FWTries to B/2, then rebooting twice: ", err)
	}

	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Rebooting: ", err)
	}
	if err := firmware.CheckFWTries(ctx, d, firmware.A, firmware.A, 0); err != nil {
		s.Error("After setting FWTries to B/2, then rebooting three times: ", err)
	}
}
