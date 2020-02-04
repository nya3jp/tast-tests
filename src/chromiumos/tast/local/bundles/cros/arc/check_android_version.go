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
		Contacts:     []string{"arc-core@google.com", "vraheja@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
	})
}

// CheckAndroidVersion Checks that 32-bit Android is not put in 64bit image unintentionally.
func CheckAndroidVersion(ctx context.Context, s *testing.State) {
	// Reuse existing ARC session.
	a := s.PreValue().(arc.PreData).ARC

	// Check compatibility for abi 64 from DUT
	abi64List, err := a.GetProp(ctx, "ro.product.cpu.abilist64")
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
	isAbi64ListEmpty := strings.Compare(abi64List, "") == 0

	// If Kernel is NOT 64-bit, or ABI List is NOT empty, Test PASS
	if !isKernel64 || !isAbi64ListEmpty {
		return
	}

	whiteListMap := map[string]struct{}{
		// Board names(case-sensitive) can be added here
		"betty-pi-arc":  struct{}{},
		"betty":         struct{}{},
		"betty-qt-arc":  struct{}{},
		"betty-arcnext": struct{}{},
		"betty-nyc-arc": struct{}{},

		"asuka":       struct{}{},
		"auron-paine": struct{}{},
		"auron-yuna":  struct{}{},
		"banon":       struct{}{},
		"bob":         struct{}{},
		"caroline":    struct{}{},
		"cave":        struct{}{},
		"celes":       struct{}{},
		"chell":       struct{}{},
		"coral":       struct{}{},
		"cyan":        struct{}{},
		"edgar":       struct{}{},
		"elm":         struct{}{},
		"gandof":      struct{}{},
		"grunt":       struct{}{},
		"hana":        struct{}{},
		"kafka":       struct{}{},
		"kevin":       struct{}{},
		"lard":        struct{}{},
		"lulu":        struct{}{},
		"poppy":       struct{}{},
		"pyro":        struct{}{},
		"reef":        struct{}{},
		"reks":        struct{}{},
		"relm":        struct{}{},
		"samus":       struct{}{},
		"sand":        struct{}{},
		"sarien":      struct{}{},
		"scarlet":     struct{}{},
		"sentry":      struct{}{},
		"setzer":      struct{}{},
		"snappy":      struct{}{},
		"terra":       struct{}{},
		"ultima":      struct{}{},
		"wizpig":      struct{}{},
	}

	// Get the board name from the DUT
	boardName, err := a.GetProp(ctx, "ro.product.name")
	if err != nil {
		s.Fatal("Failed to get the board name : ", err)
	}

	// Get the abi property from DUT
	abi, err := a.GetProp(ctx, "ro.product.cpu.abi")
	if err != nil {
		s.Fatal("Failed to get the abi property from DUT: ", err)
	}

	// Confirm that the board exists in whitelist
	if _, exists := whiteListMap[boardName]; exists != true {
		s.Logf("Board - %v must be white-listed to pass this test", boardName)
		s.Fatalf("Board is running %v kernel, but Android is %v", kernelVersion, abi)
	}
}
