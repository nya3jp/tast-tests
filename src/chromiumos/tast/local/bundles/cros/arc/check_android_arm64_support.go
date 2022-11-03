// Copyright 2020 The ChromiumOS Authors
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
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Ensures that any board with x86_64 built-in capability must support ARM64 ABI as well",
		Contacts:     []string{"arc-core@google.com", "vraheja@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func CheckAndroidARM64Support(ctx context.Context, s *testing.State) {
	// Reuse existing ARC session.
	a := s.FixtValue().(*arc.PreData).ARC

	// Check compatibility for 64-bit ABI from DUT.
	abi64List, err := a.GetProp(ctx, "ro.product.cpu.abilist64")
	if err != nil {
		s.Fatal("Failed to get the abi64 list property bytes from ARC: ", err)
	}

	isX86_64Supported := strings.Contains(abi64List, "x86_64")
	isARM64Supported := strings.Contains(abi64List, "arm64-v8a")

	s.Log("ABI 64 list = ", abi64List)
	s.Logf("x86_64 supported = %v, ARM64 supported = %v", isX86_64Supported, isARM64Supported)
	// If built-in x86_64 support is present and ARM64 support is absent, test will fail.
	if isX86_64Supported && !isARM64Supported {

		// Get the board name from the DUT.
		boardName, err := a.GetProp(ctx, "ro.product.name")
		if err != nil {
			s.Fatal("Failed to get the board name : ", err)
		}

		// Special-case for Grunt and Zork (AMD) where ARC P or earlier do not support ARM64
		if strings.Contains(boardName, "grunt") || strings.Contains(boardName, "zork") {
			sdkVer, err := arc.SDKVersion()
			if err != nil {
				s.Fatal("Failed to get the SDK version: ", err)
			}
			s.Logf("Checking AMD-based board %s on Android SDK %d", boardName, sdkVer)
			if sdkVer <= arc.SDKP {
				s.Logf("Missing ARM64 support on %s is expected for SDK %d ", boardName, sdkVer)
				return
			}
		}

		s.Fatal("Built-in x86_64 support present, but ARM 64 support absent for board - ", boardName)
	}
}
