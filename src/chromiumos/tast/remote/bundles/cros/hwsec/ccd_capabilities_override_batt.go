// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
		Func: CCDCapabilitiesOverrideBatt,
		Desc: "Test to verify OverrideBatt CCD capability locks out restricted console commands.",
		Attr: []string{"group:firmware", "group:hwsec", "firmware_ccd", "firmware_cr50"},
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"mvertescher@google.com",
		},
		Fixture:      fixture.NormalMode,
		SoftwareDeps: []string{"gsc"},
		VarDeps:      []string{"servo"},
	})
}

func CCDCapabilitiesOverrideBatt(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	err := h.OpenCCD(ctx, true, false, servo.Lock)
	if err != nil {
		s.Fatal("Failed to get open CCD: ", err)
	}

	// Turn on OverrideBatt
	err = h.Servo.SetCCDCapability(ctx, servo.OverrideBatt, servo.CapIfOpened)
	if err != nil {
		s.Fatal("Failed to set GscFullConsole capability to IfOpened", err)
	}

	// Check that we can run the `bpforce` command successfully
	err = h.SetForceBatteryPresence(ctx, true)
	if err != nil {
		s.Fatal("Failed to set force battery presence: ", err)
	}

	err = h.LockCCD(ctx)
	if err != nil {
		s.Fatal("Failed to lock ccd: ", err)
	}

	// TODO: Check for a different string on Ti50
	err = h.CheckGSCCommandOutput(ctx, "bpforce connect", []string{"Access Denied"})
	if err != nil {
		s.Fatal("`bpforce` command is not locked: ", err)
	}
}
