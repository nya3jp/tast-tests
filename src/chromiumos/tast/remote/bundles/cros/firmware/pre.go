// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	common "chromiumos/tast/common/firmware"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        Pre,
		Desc:        "Verifies firmware Preconditions",
		Contacts:    []string{"cros-fw-engprod@google.com", "aluo@google.com"},
		Data:        []string{firmware.ConfigFile},
		ServiceDeps: []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Vars:        []string{"servo"},
		Params: []testing.Param{{
			Name:      "normal",
			Val:       common.BootModeNormal,
			Pre:       pre.NormalMode(),
			ExtraAttr: []string{"group:mainline", "informational", "group:firmware", "firmware_smoke"},
		}, {
			Name: "dev",
			Val:  common.BootModeDev,
			Pre:  pre.DevMode(),
			//TODO(aluo): Re-enable when b/169704069 is resolved.
		}, {
			Name: "rec",
			Val:  common.BootModeRecovery,
			Pre:  pre.RecMode(),
			//TODO(aluo): Re-enable when b/169704069 is resolved.
		}},
	})
}

func Pre(ctx context.Context, s *testing.State) {
	v := s.PreValue().(*pre.Value)
	mode := s.Param().(common.BootMode)

	if v.BootMode != mode {
		s.Fatalf("Precondition boot mode unexpected, want %v, got %v", mode, v.BootMode)
	}

	h := v.Helper

	curr, err := h.Reporter.CurrentBootMode(ctx)
	if err != nil {
		s.Fatal("Could not report DUT boot mode: ", err)
	} else if curr != v.BootMode {
		s.Fatalf("DUT boot mode unexpected, want %v, got %v", v.BootMode, curr)
	}
	s.Logf("Successfully verified that precondition set boot mode to %q", curr)
}
