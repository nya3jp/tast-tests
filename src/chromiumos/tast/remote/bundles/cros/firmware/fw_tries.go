// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/reporters"
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
	r := reporters.New(d)

	vboot2, err := r.Vboot2(ctx)
	if err != nil {
		s.Fatal("Failed to determine fw_vboot2: ", err)
	}
	if vboot2 {
		s.Log("DUT uses vboot2")
	} else {
		s.Log("DUT does not use vboot2")
	}

	// Start the test at A/0 for consistency.
	if err := r.CheckFWTries(ctx, fwCommon.RWSectionA, fwCommon.RWSectionUnspecified, 0); err != nil {
		s.Log("Unexpected DUT state at start of test: ", err)
		if err := firmware.SetFWTries(ctx, d, fwCommon.RWSectionA, 0); err != nil {
			s.Fatal("Setting FWTries to A/0: ", err)
		}
		s.Error("After setting FWTries to A/0 at start of test: ", err)
	}

	// Set next=B, tries=2.
	if err := firmware.SetFWTries(ctx, d, fwCommon.RWSectionB, 2); err != nil {
		s.Fatal("Setting FWTries to B/2: ", err)
	}
	if err := r.CheckFWTries(ctx, fwCommon.RWSectionA, fwCommon.RWSectionB, 2); err != nil {
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
		if err := r.CheckFWTries(ctx, fwCommon.RWSectionB, fwCommon.RWSectionB, 1); err != nil {
			s.Fatal("After rebooting from Firmware A with fw_try_next=B, fw_try_count=2: ", err)
		}
		// Next reboot should use Firmware B, and decrement fw_try_count to 0.
		if err := d.Reboot(ctx); err != nil {
			s.Fatal("Rebooting: ", err)
		}
		if err := r.CheckFWTries(ctx, fwCommon.RWSectionB, fwCommon.RWSectionUnspecified, 0); err != nil {
			s.Fatal("After rebooting from Firmware B with fw_try_next=B, fw_try_count=1: ", err)
		}
		// Next reboot should return to Firmware A.
		if err := d.Reboot(ctx); err != nil {
			s.Fatal("Rebooting: ", err)
		}
		if err := r.CheckFWTries(ctx, fwCommon.RWSectionA, fwCommon.RWSectionA, 0); err != nil {
			s.Fatal("After rebooting from Firmware B with fw_try_next=A, fw_try_count=0: ", err)
		}
	} else {
		// In Vboot1, booting into Firmware B should set fwb_tries=0.
		if err := r.CheckFWTries(ctx, fwCommon.RWSectionB, fwCommon.RWSectionA, 0); err != nil {
			s.Fatal("After rebooting from Firmware A with fwb_tries=2: ", err)
		}
		// Next reboot should return to Firmware A.
		if err := d.Reboot(ctx); err != nil {
			s.Fatal("Rebooting: ", err)
		}
		if err := r.CheckFWTries(ctx, fwCommon.RWSectionA, fwCommon.RWSectionA, 0); err != nil {
			s.Fatal("After rebooting from firmware B with fwb_tries=0: ", err)
		}
	}
}
