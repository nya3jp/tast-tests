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

	currentFW, nextFW, tryCount, err := r.FWTries(ctx)
	if err != nil {
		s.Fatal("Reporting FW Tries at start of test: ", err)
	}
	s.Logf("At start of test, currentFW/nextFW/tryCount are: %s/%s/%d", currentFW, nextFW, tryCount)

	// Set next=B, tries=2.
	if err := firmware.SetFWTries(ctx, d, fwCommon.RWSectionB, 2); err != nil {
		s.Fatal("Setting FWTries to B/2: ", err)
	}
	if err := firmware.CheckFWTries(ctx, r, fwCommon.RWSectionUnspecified, fwCommon.RWSectionB, 2); err != nil {
		s.Fatal("After setting FWTries to B/2, before rebooting: ", err)
	}
	s.Log("nextFW/tryCount has been set to B/2")

	// Reboot the DUT.
	s.Log("Rebooting; expect to boot into B leaving tryCount=1 or 0")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Rebooting: ", err)
	}

	// DUT should have rebooted into firmware B, and tryCount should have decremented by 1.
	// Occasionally, vboot needs an extra reboot along the way, so the tryCount decrements by 2 instead. This is OK.
	currentFW, nextFW, tryCount, err = r.FWTries(ctx)
	if err != nil {
		s.Fatal("Reporting FW tries after first reboot: ", err)
	}
	if currentFW != fwCommon.RWSectionB {
		s.Fatalf("After rebooting from A/B/2: unexpected currentFW: got %s; want B", currentFW)
	}
	if nextFW == fwCommon.RWSectionB && tryCount == 1 {
		s.Log("DUT rebooted once. currentFW/nextFW/tryCount: B/B/1")
		s.Log("Rebooting; expect to boot into B leaving tryCount=0")
		if err := d.Reboot(ctx); err != nil {
			s.Fatal("Rebooting: ", err)
		}
		if err := firmware.CheckFWTries(ctx, r, fwCommon.RWSectionB, fwCommon.RWSectionUnspecified, 0); err != nil {
			s.Fatal("After rebooting from B/B/1: ", err)
		}
	} else if nextFW == fwCommon.RWSectionA && tryCount == 0 {
		s.Log("DUT rebooted twice. currentFW/nextFW/tryCount: B/A/0")
	} else {
		s.Fatalf("After setting FWTries to B/2 then rebooting: unexpected nextFW/tryCount: got %s/%d; want B/1 or A/0", nextFW, tryCount)
	}

	// Next reboot should return to Firmware A.
	s.Log("Rebooting; expect to boot into A leaving tryCount=0")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Rebooting: ", err)
	}
	if err := firmware.CheckFWTries(ctx, r, fwCommon.RWSectionA, fwCommon.RWSectionA, 0); err != nil {
		s.Fatal("After rebooting from B/A/0: ", err)
	}
}
