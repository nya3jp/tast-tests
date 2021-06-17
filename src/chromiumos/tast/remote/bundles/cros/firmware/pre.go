// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	common "chromiumos/tast/common/firmware"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/checkers"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Pre,
		Desc:         "Verifies firmware Preconditions",
		Contacts:     []string{"cros-fw-engprod@google.com", "aluo@google.com"},
		Data:         []string{firmware.ConfigFile},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		Vars:         []string{"servo"},
		Attr:         []string{"group:firmware"},
		Params: []testing.Param{{
			Name:      "normal",
			Val:       common.BootModeNormal,
			Pre:       pre.NormalMode(),
			ExtraAttr: []string{"firmware_smoke"},
		}, {
			Name: "dev",
			Val:  common.BootModeDev,
			Pre:  pre.DevMode(),
			// TODO(aluo): Re-enable when b/169704069 is resolved.
		}, {
			Name:      "rec",
			Val:       common.BootModeRecovery,
			Pre:       pre.RecMode(),
			ExtraAttr: []string{"firmware_smoke"},
		}},
	})
}

func Pre(ctx context.Context, s *testing.State) {
	v := s.PreValue().(*pre.Value)
	mode := s.Param().(common.BootMode)

	if v.BootMode != mode {
		s.Fatalf("Precondition boot mode unexpected, got %v, want %v", v.BootMode, mode)
	}

	h := v.Helper

	curr, err := h.Reporter.CurrentBootMode(ctx)
	if err != nil {
		s.Error("Could not report DUT boot mode: ", err)
	} else if curr != v.BootMode {
		s.Errorf("DUT boot mode unexpected, got %v, want %v", curr, v.BootMode)
	} else {
		s.Logf("Successfully verified that precondition set boot mode to %q", v.BootMode)
	}

	checker := checkers.New(h)

	if err := checker.GBBFlags(ctx, v.GBBFlags); err != nil {
		s.Fatal("DUT GBB flags incorrect: ", err)
	} else {
		s.Logf("Successfully verified that precondition set GBB flags to %q", v.GBBFlags)
	}
}
