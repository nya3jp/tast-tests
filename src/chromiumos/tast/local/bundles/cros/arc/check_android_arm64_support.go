// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CheckAndroidARM64Support,
		Desc:         "Ensures that any board with x86_64 native capability must support ARM64 ABI as well",
		Contacts:     []string{"arc-core@google.com", "vraheja@google.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func isInExceptionMap(boardName string) bool {
	// Board names(case-sensitive) can be added to this map.
	exceptionMap := map[string]struct{}{
		"eve": {},
	}

	// Confirm that the board exists in the exception map.
	_, exists := exceptionMap[boardName]
	return exists
}

func CheckAndroidARM64Support(ctx context.Context, s *testing.State) {
	// Reuse existing ARC session.
	a := s.PreValue().(arc.PreData).ARC

	// Check compatibility for 64-bit ABI from DUT.
	abi64List, err := a.GetProp(ctx, "ro.product.cpu.abilist64")
	if err != nil {
		s.Fatal("Failed to get the abi64 list property bytes from ARC: ", err)
	}

	isX86_64Supported := strings.Contains(abi64List, "x86_64")
	isARM64Supported := strings.Contains(abi64List, "arm64-v8a")

	s.Log("ABI 64 list = ", abi64List)
	s.Logf("x86_64 supported = %v, ARM64 supported = %v", isX86_64Supported, isARM64Supported)
	// If Native x86_64 support is present and ARM64 support is absent, board must be explicitly allowed.
	if isX86_64Supported && !isARM64Supported {

		// Get the board name from the DUT.
		boardName, err := a.GetProp(ctx, "ro.product.name")
		if err != nil {
			s.Fatal("Failed to get the board name : ", err)
		}

		isBoardAllowed := isInExceptionMap(boardName)
		if isBoardAllowed == false {
			s.Logf("Board - %v must be explicitly allowed to pass this test", boardName)
			s.Fatal("Native x86_64 support present, but ARM 64 support absent for board - ", boardName)
		}

	}
}
