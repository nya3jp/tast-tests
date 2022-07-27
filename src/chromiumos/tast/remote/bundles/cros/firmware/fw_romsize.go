// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing/hwdep"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FWROMSize,
		Desc:         "Check that flash sizes are within reasonable range",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Fixture:      fixture.NormalMode,
	})
}

// FWROMSize simply checks if AP and EC chips are above their
// minimum required values to prevent improper readings w/ flashrom
func FWROMSize(ctx context.Context, s *testing.State) {

	const (
		minECSize = 512
		minAPSize = 4096
	)

	h := s.FixtValue().(*fixture.Value).Helper

	apSizeStr, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", "flashrom --get-size -p host | tail -n1").Output()
	if err != nil {
		s.Fatal("Failed to determine AP firmware size: ", err)
	}

	ecSizeStr, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", "flashrom --get-size -p ec | tail -n1").Output()
	if err != nil {
		s.Fatal("Failed to determine EC firmware size: ", err)
	}

	apSize, err := strconv.Atoi(strings.TrimSuffix(string(apSizeStr), "\n"))
	if err != nil {
		s.Fatal("Failed to parse AP firmware size as integer: ", err)
	}

	ecSize, err := strconv.Atoi(strings.TrimSuffix(string(ecSizeStr), "\n"))
	if err != nil {
		s.Fatal("Failed to parse EC firmware size as integer: ", err)
	}

	s.Log("AP firmware size in kilobytes: ", apSize/1024)
	s.Log("EC firmware size in kilobytes: ", ecSize/1024)
	if (apSize / 1024) < minAPSize {
		s.Fatalf("AP firmware size is less than expected: want >-%d KB, got %d KB", minAPSize, apSize/1024)
	}

	if (ecSize / 1024) < minECSize {
		s.Fatalf("AP firmware size is less than expected: want >-%d KB, got %d KB", minECSize, ecSize/1024)
	}
}
