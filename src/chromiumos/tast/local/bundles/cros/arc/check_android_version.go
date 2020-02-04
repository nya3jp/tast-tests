// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CheckAndroidVersion,
		Desc:         "Checks that we are not shipping 32-bit Android on a 64-bit Kernel unintentionally",
		Contacts:     []string{"vraheja@google.com"},
		SoftwareDeps: []string{"android_all_both", "chrome"},
		Pre:          arc.Booted(),
		Attr:         []string{"group:mainline", "informational"},
	})
}

// CheckAndroidVersion ...Checks that 32-bit Android is not put in 64bit image unintentionally
func CheckAndroidVersion(ctx context.Context, s *testing.State) {
	// Reuse existing ARC session.
	a := s.PreValue().(arc.PreData).ARC

	// Check compatibility for abi 64 from DUT
	prop, err := a.GetProp(ctx, "ro.product.cpu.abilist64")
	if err != nil {
		s.Fatal("Failed to get the abi64 list property bytes from the Android: ", err)
	}

	// Get the Kernel version from the DUT
	kernelVersionByte, err := testexec.CommandContext(ctx,
		"uname", "-m").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get the Kernel Version: ", err)
	}
	kernelVersion := strings.TrimSpace(string(kernelVersionByte))
	isKernel64 := strings.Contains(kernelVersion, "64")
	isAbi64ListEmpty := strings.Compare(prop, "") == 0

	// If both the flags are true, board must be white-listed to pass the test
	if isKernel64 && isAbi64ListEmpty {

		whiteListMap := map[string]struct{}{
			// Board names(case-sensitive) can be added here
			"grunt":  struct{}{},
			"sarien": struct{}{},
		}

		// Get the board name from the DUT
		boardName, err := a.GetProp(ctx, "ro.product.name")
		if err != nil {
			s.Fatal("Failed to get the board name : ", err)
		}

		// Get the abi property from DUT
		prop32, err := a.GetProp(ctx, "ro.product.cpu.abi")
		if err != nil {
			s.Fatal("Failed to get the abi property from DUT: ", err)
		}

		// Confirm that the board exists in whitelist
		if _, exists := whiteListMap[boardName]; exists != true {
			s.Logf("Board - %v must be white-listed to pass this test", boardName)
			s.Fatalf("Board is running %v kernel, but Android is %v", kernelVersion, prop32)
		}
	}
}
