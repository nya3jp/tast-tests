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
		Attr:         []string{"group:mainline", "informational", "group:firmware", "firmware_smoke"},
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

	// Start the test at A/0 for consistency.
	if err := firmware.CheckFWTries(ctx, d, firmware.A, firmware.UnspecifiedRWSection, 0); err != nil {
		s.Log("Unexpected DUT state at start of test: ", err)
		if err := firmware.SetFWTries(ctx, d, firmware.A, 0); err != nil {
			s.Fatal("Setting FWTries to A/0: ", err)
		}
		s.Error("After setting FWTries to A/0 at start of test: ", err)
	}

	// Set next=B, tries=2.
	if err := firmware.SetFWTries(ctx, d, firmware.B, 2); err != nil {
		s.Fatal("Setting FWTries to B/2: ", err)
	}
	if err := firmware.CheckFWTries(ctx, d, firmware.A, firmware.B, 2); err != nil {
		s.Error("After setting FWTries to B/2, before rebooting: ", err)
	}

	// Reboot the DUT.
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Rebooting: ", err)
	}

	// Post-reboot behavior changes based on the Vboot version.
	if vboot2 {
		// In Vboot2, booting into Firmware B should decrement fw_try_count.
		// fw_try_next should not change unless fw_try_count reaches 0.
		// So the DUT should now be in Firmware B with fw_try_count=1.
		if err := firmware.CheckFWTries(ctx, d, firmware.B, firmware.B, 1); err != nil {
			s.Fatal("After rebooting from Firmware A with fw_try_next=B, fw_try_count=2: ", err)
		}
		// Next reboot should use Firmware B, and decrement fw_try_count to 0.
		if err := d.Reboot(ctx); err != nil {
			s.Fatal("Rebooting: ", err)
		}
		if err := firmware.CheckFWTries(ctx, d, firmware.B, firmware.UnspecifiedRWSection, 0); err != nil {
			s.Fatal("After rebooting from Firmware B with fw_try_next=B, fw_try_count=1: ", err)
		}
		// Next reboot should return to Firmware A.
		if err := d.Reboot(ctx); err != nil {
			s.Fatal("Rebooting: ", err)
		}
		if err := firmware.CheckFWTries(ctx, d, firmware.A, firmware.A, 0); err != nil {
			s.Fatal("After rebooting from Firmware B with fw_try_next=A, fw_try_count=0: ", err)
		}
	} else {
		// In Vboot1, booting into Firmware B should set fwb_tries=0.
		if err := firmware.CheckFWTries(ctx, d, firmware.B, firmware.A, 0); err != nil {
			s.Fatal("After rebooting from Firmware A with fwb_tries=2: ", err)
		}
		// Next reboot should return to Firmware A.
		if err := d.Reboot(ctx); err != nil {
			s.Fatal("Rebooting: ", err)
		}
		if err := firmware.CheckFWTries(ctx, d, firmware.A, firmware.A, 0); err != nil {
			s.Fatal("After rebooting from firmware.B with fwb_tries=0: ", err)
		}
	}
}
