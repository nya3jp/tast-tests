// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FWTries,
		Desc:         "Verify that the DUT can be specified to boot from A or B",
		Contacts:     []string{"cros-fw-engprod@google.com"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Attr:         []string{"group:firmware", "firmware_bios"},
		Vars:         []string{"servo"},
		Params: []testing.Param{
			testing.Param{
				Name:    "normal",
				Fixture: fixture.NormalMode,
			},
			testing.Param{
				Name:    "dev",
				Fixture: fixture.DevModeGBB,
			},
		},
	})
}

func FWTries(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Creating mode switcher: ", err)
	}

	vboot2, err := h.Reporter.Vboot2(ctx)
	if err != nil {
		s.Fatal("Failed to determine fw_vboot2: ", err)
	}
	if vboot2 {
		s.Log("DUT uses vboot2")
	} else {
		s.Log("DUT does not use vboot2")
	}

	currentFW, nextFW, tryCount, err := h.Reporter.FWTries(ctx)
	if err != nil {
		s.Fatal("Reporting FW Tries at start of test: ", err)
	}
	s.Logf("At start of test, currentFW/nextFW/tryCount are: %s/%s/%d", currentFW, nextFW, tryCount)

	// Set next=B, tries=2.
	if err := firmware.SetFWTries(ctx, h.DUT, fwCommon.RWSectionB, 2); err != nil {
		s.Fatal("Setting FWTries to B/2: ", err)
	}
	if err := firmware.CheckFWTries(ctx, h.Reporter, fwCommon.RWSectionUnspecified, fwCommon.RWSectionB, 2); err != nil {
		s.Fatal("After setting FWTries to B/2, before rebooting: ", err)
	}
	s.Log("nextFW/tryCount has been set to B/2")

	// Reboot the DUT.
	s.Log("Rebooting; expect to boot into B leaving tryCount=1 or 0")
	if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
		s.Fatal("Error resetting DUT: ", err)
	}

	// DUT should have rebooted into firmware B, and tryCount should have decremented by 1.
	// Occasionally, vboot needs an extra reboot along the way, so the tryCount decrements by 2 instead. This is OK.
	currentFW, nextFW, tryCount, err = h.Reporter.FWTries(ctx)
	if err != nil {
		s.Fatal("Reporting FW tries after first reboot: ", err)
	}
	if currentFW != fwCommon.RWSectionB {
		s.Fatalf("After rebooting from A/B/2: unexpected currentFW: got %s; want B", currentFW)
	}
	if nextFW == fwCommon.RWSectionB && tryCount == 1 {
		s.Log("DUT rebooted once. currentFW/nextFW/tryCount: B/B/1")
		s.Log("Rebooting; expect to boot into B leaving tryCount=0")
		if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
			s.Fatal("Error resetting DUT: ", err)
		}
		if err := firmware.CheckFWTries(ctx, h.Reporter, fwCommon.RWSectionB, fwCommon.RWSectionUnspecified, 0); err != nil {
			s.Fatal("After rebooting from B/B/1: ", err)
		}
		currentFW, nextFW, tryCount, err = h.Reporter.FWTries(ctx)
		s.Logf("DUT rebooted. currentFW/nextFW/tryCount:%s/%s/%d", currentFW, nextFW, tryCount)
	} else if tryCount == 0 {
		s.Logf("DUT rebooted twice. currentFW/nextFW/tryCount: B/%s/0", nextFW)
	} else {
		s.Fatalf("After setting FWTries to B/2 then rebooting: unexpected nextFW/tryCount: got %s/%d; want B/1 or {either}/0", nextFW, tryCount)
	}

	// Next reboot should return to Firmware A.
	s.Log("Rebooting; expect to boot into A leaving tryCount=0")
	if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
		s.Fatal("Error resetting DUT: ", err)
	}
	if err := firmware.CheckFWTries(ctx, h.Reporter, fwCommon.RWSectionA, fwCommon.RWSectionA, 0); err != nil {
		s.Fatal("After rebooting from B/A/0: ", err)
	}
}
