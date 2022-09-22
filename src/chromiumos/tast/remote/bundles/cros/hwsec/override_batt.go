// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OverrideBatt,
		Desc: "Test to verify OverrideBatt CCD capability locks out restricted console commands.",
		Attr: []string{"group:firmware", "group:hwsec_ccd_capabilities", "firmware_cr50"},
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"mvertescher@google.com",
		},
		Fixture:      fixture.NormalMode,
		SoftwareDeps: []string{"gsc"},
		VarDeps:      []string{"servo"},
	})
}

func OverrideBatt(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	err := h.OpenCCD(ctx, true, true, servo.Lock)
	if err != nil {
		s.Fatal("Failed to get open CCD: ", err)
	}

	// Ensure CCD is open
	ccdLevel, err := h.GetCCDLevel(ctx)
	if err != nil {
		s.Fatal("Failed to get CCD level: ", err)
	}
	if ccdLevel != "open" {
		s.Fatal("Failed to open CCD, state = ", ccdLevel)
	}

	// Turn on OverrideBatt if open
	err = h.Servo.SetCCDCapability(ctx, servo.OverrideBatt, servo.CapIfOpened)
	if err != nil {
		s.Fatal("Failed to set GscFullConsole capability to IfOpened", err)
	}

	// Check that we can run bpforce
	matches, err := h.Servo.RunCR50CommandGetOutput(ctx, "bpforce con", []string{`Access Denied`})
	if err != nil {
		s.Fatal("Failed to run bpforce command", err)
	}
	if len(matches) != 0 {
		s.Fatal("Expected bpforce command to run successfully, but it is disabled")
	}

	// Lock CCD
	err = h.Servo.RunCR50Command(ctx, "ccd lock")
	if err != nil {
		s.Fatal("Failed to lock ccd: ", err)
	}

	// TODO: Running locked commmands fail with a generic communication error
	// and we can't get text output to verify that access is denied.

	// Check that we can't run idle
	matches, err = h.Servo.RunCR50CommandGetOutput(ctx, "bpforce con", []string{`*`}) // Console is locked*`, `*Access Denied`})
	if err == nil {
		s.Fatal("Expected bpforce command to fail because CCD is locked, but it ran successfully")
	}
}
