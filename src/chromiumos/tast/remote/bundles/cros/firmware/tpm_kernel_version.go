// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TPMKernelVersion,
		Desc:         "Check firmware and kernel version stored in TPM",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Fixture:      fixture.DevMode,
	})
}

// TPMKernelVersion verifies that kernel and firmware version stored
// in TPM are read properly and not containing invalid values.
func TPMKernelVersion(ctx context.Context, s *testing.State) {

	h := s.FixtValue().(*fixture.Value).Helper

	fwVersionStr, err := h.DUT.Conn().CommandContext(ctx, "crossystem", "tpm_fwver").Output()
	if err != nil {
		s.Fatal("Failed to determine AP firmware size: ", err)
	}

	kernVersionStr, err := h.DUT.Conn().CommandContext(ctx, "crossystem", "tpm_kernver").Output()
	if err != nil {
		s.Fatal("Failed to determine EC firmware size: ", err)
	}

	fwVersion, err := strconv.ParseInt(
		strings.Replace(string(fwVersionStr), "0x", "", -1),
		16, 64,
	)
	if err != nil {
		s.Fatal("Failed to parse firmware version stored in TPM as HEX string: ", err)
	}

	kernVersion, err := strconv.ParseInt(
		strings.Replace(string(kernVersionStr), "0x", "", -1),
		16, 64,
	)
	if err != nil {
		s.Fatal("Failed to parse kernel version stored in TPM as string: ", err)
	}

	s.Logf("Kernel version in TPM: 0x%x", kernVersion)
	s.Logf("Firmware version in TPM: 0x%x", fwVersion)

	if kernVersion == 0xFFFFFFFF {
		s.Fatal("Invalid kernel version found in TPM!")
	}

	if fwVersion == 0xFFFFFFFF {
		s.Fatal("Invalid firmware version found in TPM!")
	}
}
