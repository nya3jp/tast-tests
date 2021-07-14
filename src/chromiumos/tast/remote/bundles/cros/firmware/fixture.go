// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	common "chromiumos/tast/common/firmware"
	"chromiumos/tast/remote/firmware/checkers"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Fixture,
		Desc:         "Verifies firmware fixtures",
		Contacts:     []string{"cros-fw-engprod@google.com", "jbettis@google.com"},
		Attr:         []string{"group:firmware", "firmware_smoke"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		SoftwareDeps: []string{"crossystem"},
		Params: []testing.Param{{
			Name:    "normal",
			Val:     common.BootModeNormal,
			Fixture: fixture.NormalMode,
		}, {
			Name:    "dev",
			Val:     common.BootModeDev,
			Fixture: fixture.DevMode,
		}, {
			Name:    "dev_gbb",
			Val:     common.BootModeDev,
			Fixture: fixture.DevModeGBB,
		}, {
			Name:      "rec",
			Val:       common.BootModeRecovery,
			Fixture:   fixture.RecMode,
			ExtraAttr: []string{"firmware_usb"},
		}},
	})
}

func Fixture(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)
	wantMode := s.Param().(common.BootMode)

	if v.BootMode != wantMode {
		s.Errorf("Unexpected fixture boot mode: got %q, want %q", v.BootMode, wantMode)
	}

	h := v.Helper

	curr, err := h.Reporter.CurrentBootMode(ctx)
	if err != nil {
		s.Error("Could not report DUT boot mode: ", err)
	} else if curr != wantMode {
		s.Errorf("Unexpected DUT boot mode: got %q, want %q", curr, v.BootMode)
	}

	checker := checkers.New(h)

	if err := checker.GBBFlags(ctx, v.GBBFlags); err != nil {
		s.Error("Checker: ", err)
	}
}
