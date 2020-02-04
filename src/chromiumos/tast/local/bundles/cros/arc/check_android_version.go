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

// giveBoardName ...Fetches the board name from the machine
func giveBoardName(ctx context.Context, s *testing.State) string {
	boardByte, err := testexec.CommandContext(ctx,
		"grep", "CHROMEOS_RELEASE_BOARD", "/etc/lsb-release").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get the board name: ", err)
	}

	var boardName = string(boardByte)
	boardName = strings.TrimLeft(boardName, "CHROMEOS_RELEASE_BOARD=")
	boardName = strings.TrimSpace(boardName)

	return boardName
}

// CheckAndroidVersion ...Checks that 32-bit Android is not put in 64bit image unintentionally
func CheckAndroidVersion(ctx context.Context, s *testing.State) {
	// Reuse existing ARC session.
	a := s.PreValue().(arc.PreData).ARC

	// Get the android version from the image using the Getprop through ADB
	propByte, err := a.GetProp(ctx, "ro.product.cpu.abilist64")
	var propStr = string(propByte)
	if err != nil {
		s.Fatal("Failed to get the abi64 list property bytes from the Android: ", err)
	}
	// Get the Kernel version from the image
	kernelVersionByte, err := testexec.CommandContext(ctx,
		"uname", "-m").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get the Kernel Version: ", err)
	}
	var kernelVersion = string(kernelVersionByte)
	var isKernel64 = strings.Contains(kernelVersion, "64")
	var isAbi64ListEmpty = strings.Compare(propStr, "") == 0

	// If both the flags are true, board(lowercase) must be white-listed to pass the test
	if isKernel64 && isAbi64ListEmpty {

		whiteListMap := make(map[string]struct{})
		// Board name can be added here
		whiteListMap["grunt"] = struct{}{}

		var boardName = giveBoardName(ctx, s)
		_, exists := whiteListMap[boardName]
		if exists != true {
			s.Fatal("Wrong Android getting shipped in the board - ", boardName)
		}
	}
}
