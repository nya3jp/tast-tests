// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FWTries,
		Desc:         "Verify that the DUT can be specified to boot from A or B",
		Contacts:     []string{"cros-fw-engprod@google.com"},
		SoftwareDeps: []string{"crossystem"},
		Attr:         []string{"group:firmware", "firmware_smoke"},
		Vars:         []string{"servo"},
	})
}

func FWTries(ctx context.Context, s *testing.State) {
	d := s.DUT()
	r := reporters.New(d)

	// Sometimes the reboots error, leaving the DUT powered-off at end-of-test.
	// This prevents Tast from reconnecting to the DUT for future tests, and from reporting results after all tests are finished.
	// To address this, use Servo to defer a power-mode reset at end-of-test.
	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		if d.Connected(ctx) {
			return
		}
		s.Log("DUT not connected at end-of-test. Cold-resetting")
		servoSpec, _ := s.Var("servo")
		pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
		if err != nil {
			s.Fatal("Failed to connect to servo: ", err)
		}
		defer pxy.Close(ctx)
		if err := pxy.Servo().SetPowerState(ctx, servo.PowerStateReset); err != nil {
			s.Fatal("Resetting DUT during cleanup: ", err)
		}
		if err := d.WaitConnect(ctx); err != nil {
			s.Fatal("Reconnecting to DUT during cleanup: ", err)
		}
	}(ctxForCleanup)

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
	} else if tryCount == 0 {
		s.Logf("DUT rebooted twice. currentFW/nextFW/tryCount: B/%s/0", nextFW)
	} else {
		s.Fatalf("After setting FWTries to B/2 then rebooting: unexpected nextFW/tryCount: got %s/%d; want B/1 or {either}/0", nextFW, tryCount)
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
