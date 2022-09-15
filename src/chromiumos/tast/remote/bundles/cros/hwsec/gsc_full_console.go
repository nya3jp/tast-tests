// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GscFullConsole,
		Desc: "Test to verify GscFullConsole locks out restricted console commands.",
		Attr: []string{"group:firmware", "firmware_cr50"},
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"mvertescher@google.com",
		},
		Fixture:      fixture.NormalMode,
		SoftwareDeps: []string{"gsc"},
		Timeout:      2 * time.Minute,
		VarDeps:      []string{"servo"},
	})
}

func GscFullConsole(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	// Ensure CCD is open
	err := h.OpenCCD(ctx, true, true, servo.Lock)
	if err != nil {
		s.Fatal("Failed to get open CCD: ", err)
	}

	ccdLevel, err := h.GetCCDLevel(ctx)
	if err != nil {
		s.Fatal("Failed to get CCD level: ", err)
	}
	if ccdLevel != "open" {
		s.Fatal("Failed to open CCD, state = ", ccdLevel)
	}

	// Turn on GscFullConsole if open
	err = h.Servo.SetCCDCapability(ctx, servo.GscFullConsole, servo.CapIfOpened)
	if err != nil {
		s.Fatal("Failed to set GscFullConsole capability to Always", err)
	}

	// Check that we can run idle
	matches, err := h.Servo.RunCR50CommandGetOutput(ctx, "idle s", []string{`idle action: sleep`})
	if err != nil {
		s.Fatal("Failed to run idle command", err)
	}
	if len(matches) != 1 {
		s.Fatal("idle command is not disabled")
	}

	// Check that we can run recbtnforce
	matches, err = h.Servo.RunCR50CommandGetOutput(ctx, "recbtnforce enable", []string{`RecBtn:*`})
	if err != nil {
		s.Fatal("Failed to run recbtnforce command", err)
	}
	if len(matches) != 1 {
		s.Fatal("recbtnforce command is not disabled")
	}

	// Check that we can run rddkeepalive
	matches, err = h.Servo.RunCR50CommandGetOutput(ctx, "rddkeepalive true", []string{`Forcing*`})
	if err != nil {
		s.Fatal("Failed to run rddkeepalive command", err)
	}
	if len(matches) != 1 {
		s.Fatal("rddkeepalive command is not disabled")
	}

	// Lock CCD
	err = h.Servo.RunCR50Command(ctx, "ccd lock")
	if err != nil {
		s.Fatal("Failed to lock ccd: ", err)
	}

	// TODO: Running locked commands fail with a generic communication error
	// and we can't get text output to verify that access is denied.

	// Check that we can't run idle
	matches, err = h.Servo.RunCR50CommandGetOutput(ctx, "idle s", []string{`*`}) // Console is locked*`, `*Access Denied`})
	if err == nil {
		s.Fatal("Expected idle command to fail because CCD is locked, but it ran successfully")
	}

	// Check that we can't run recbtnforce
	matches, err = h.Servo.RunCR50CommandGetOutput(ctx, "recbtnforce enable", []string{`*Access Denied`})
	if err == nil {
		s.Fatal("Expected recbtnforce command to fail because CCD is locked, but it ran successfully")
	}

	// Check that we can't run rddkeepalive
	matches, err = h.Servo.RunCR50CommandGetOutput(ctx, "rddkeepalive true", []string{`*Access Denied`})
	if err == nil {
		s.Fatal("Expected rddkeepalive command to fail because CCD is locked, but it ran successfully")
	}
}
