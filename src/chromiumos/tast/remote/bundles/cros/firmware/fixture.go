// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	common "chromiumos/tast/common/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Fixture,
		Desc:         "Verifies firmware fixtures",
		Contacts:     []string{"cros-fw-engprod@google.com", "jbettis@google.com"},
		Attr:         []string{"group:firmware", "firmware_smoke"},
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
			Name:      "dev_usb",
			Val:       common.BootModeUSBDev,
			Fixture:   fixture.USBDevModeNoServices,
			ExtraAttr: []string{"firmware_usb"},
		}, {
			Name:      "dev_usb_gbb",
			Val:       common.BootModeUSBDev,
			Fixture:   fixture.USBDevModeGBBNoServices,
			ExtraAttr: []string{"firmware_usb"},
		}, {
			Name:      "rec",
			Val:       common.BootModeRecovery,
			Fixture:   fixture.RecModeNoServices,
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

	if res, err := common.GetGBBFlags(ctx, h.DUT); err != nil {
		s.Error("Failed to get GBB flags: ", err)
	} else if !common.GBBFlagsStatesEqual(v.GBBFlags, res) {
		s.Errorf("GBB flags: got %v, want %v", res.Set, v.GBBFlags)
	}
}
