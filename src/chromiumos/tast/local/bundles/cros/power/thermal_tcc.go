// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ThermalTCC,
		Desc: "Check that TCC and TCC offset value are correct and not changed after suspend",
		Contacts: []string{
			"puthik@chromium.org",                // test author
			"chromeos-platform-power@google.com", // CrOS platform power developers
		},
		Attr: []string{"group:mainline", "informational"},
		// Only applied to newer Intel boards
		HardwareDeps: hwdep.D(hwdep.Platform("poppy", "nami", "hatch", "volteer", "brya", "dedede")),
	})
}

func readTCC(ctx context.Context) (int, int, error) {
	// Read MSR at 0x1a2h for TCC and TCC offset
	out, err := testexec.CommandContext(ctx, "iotools", "rdmsr", "0", "0x1a2h").Output()
	if err != nil {
		return 0, 0, errors.Wrap(err, "can't read MSR 0x1a2h")
	}
	rawData, err := strconv.ParseInt(strings.TrimSuffix(string(out), "\n"), 0, 64)
	if err != nil {
		return 0, 0, errors.Wrap(err, "can't parse TCC to number")
	}

	// TCC offset is bit 29:24
	tccOffset := (rawData & 0x3f000000) >> 24
	// TCC is bit 23:16
	tcc := (rawData & 0xff0000) >> 16

	return int(tcc), int(tccOffset), nil
}

func ThermalTCC(ctx context.Context, s *testing.State) {
	// Check that TCC and TCC offset match common values
	tcc, tccOffset, err := readTCC(ctx)
	if err != nil {
		s.Fatal("Can't read TCC: ", err)
	}
	if tcc < 80 {
		s.Errorf("TCC is %dC, want >= 80C", tcc)
	}
	if tccOffset < 2 {
		s.Errorf("TCC offset is %dC, want >= 2C", tccOffset)
	}

	// Suspend for 5 seconds
	err = testexec.CommandContext(ctx, "powerd_dbus_suspend", "--suspend_for_sec=5").Run()
	if err != nil {
		s.Fatal("Failed to suspend", tccOffset)
	}

	// Check that values don't change
	tccNew, tccOffsetNew, err := readTCC(ctx)
	if err != nil {
		s.Fatal("Can't read TCC: ", err)
	}
	if tccNew != tcc {
		s.Errorf("TCC changes after suspend, before: %dC after: %dC", tcc, tccNew)
	}
	if tccOffsetNew != tccOffset {
		s.Errorf("TCC offset changes after suspend, before: %dC after: %dC", tccOffset, tccOffsetNew)
	}
}
