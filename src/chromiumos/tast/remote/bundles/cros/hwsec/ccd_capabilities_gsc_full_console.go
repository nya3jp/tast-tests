// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
		Func: CCDCapabilitiesGscFullConsole,
		Desc: "Test to verify GscFullConsole locks out restricted console commands.",
		Attr: []string{"group:firmware", "group:hwsec", "firmware_ccd", "firmware_cr50"},
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

func CCDCapabilitiesGscFullConsole(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	err := h.OpenCCD(ctx, true, false, servo.Lock)
	if err != nil {
		s.Fatal("Failed to get open CCD: ", err)
	}

	err = h.Servo.SetCCDCapability(ctx, servo.GscFullConsole, servo.CapIfOpened)
	if err != nil {
		s.Fatal("Failed to set `GscFullConsole` capability to `Always`: ", err)
	}

	err = h.CheckGSCCommandOutput(ctx, "idle s", []string{`idle action: sleep`})
	if err != nil {
		s.Fatal("Failed to run `idle` command successfully: ", err)
	}

	err = h.CheckGSCCommandOutput(ctx, "recbtnforce enable", []string{`RecBtn:`})
	if err != nil {
		s.Fatal("Failed to run `recbtnforce` command successfully: ", err)
	}

	err = h.CheckGSCCommandOutput(ctx, "rddkeepalive true", []string{`Forcing`})
	if err != nil {
		s.Fatal("Failed to run `rddkeepalive` command successfully: ", err)
	}

	err = h.LockCCD(ctx)
	if err != nil {
		s.Fatal("Failed to lock ccd: ", err)
	}

	err = h.CheckGSCCommandOutput(ctx, "idle s", []string{`Console is locked|Access Denied`})
	if err != nil {
		s.Fatal("Failed to ensure `idle` command failed: ", err)
	}

	err = h.CheckGSCCommandOutput(ctx, "recbtnforce enable", []string{`Access Denied`})
	if err != nil {
		s.Fatal("Failed to ensure `recbtnforce` command failed: ", err)
	}

	err = h.CheckGSCCommandOutput(ctx, "rddkeepalive true", []string{`Parameter 1 invalid|Access Denied`})
	if err != nil {
		s.Fatal("Failed to ensure `rddkeepalive` command failed: ", err)
	}
}
