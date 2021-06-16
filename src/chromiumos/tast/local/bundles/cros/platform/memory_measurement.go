// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/memory/mempressure"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

type memoryMeasureParams struct {
	enableARC bool
	enableSteam bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     MemoryMeasurement,
		Desc:     "Log in to Chrome OS and measure memory usage",
		Contacts: []string{"sonnyrao@chromium.org", "chromeos-memory@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_memory_nightly"},
		Timeout:  10 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: &memoryMeasureParams{enableARC: false},
		}, {
			Name:              "arcvm",
			Val:               &memoryMeasureParams{enableARC: true},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "container",
			Val:               &memoryMeasureParams{enableARC: true},
			ExtraSoftwareDeps: []string{"android_p"},
		}},
	})
}

// MemoryMeasurement is the main test function.
func MemoryMeasurement(ctx context.Context, s *testing.State) {
	enableARC := s.Param().(*memoryMeasureParams).enableARC

	// Clean up old swap -- disable and renable swap
	// Hack for now assume zram
	swapoff_cmd := testexec.CommandContext(ctx, "swapoff", "/dev/zram0")
	//swpoff_out, swpoff_err := swapoff_cmd.Output()
	swapoff_cmd.Run()
	swapon_cmd := testexec.CommandContext(ctx, "swapon", "/dev/zram0")
	//swpon_out, swpon_err := swapon_cmd.Output()
	swapon_cmd.Run()
	
	testEnv, err := mempressure.NewTestEnv(ctx, s.OutDir(), enableARC, false,
		"")
	if err != nil {
	 	s.Fatal("Failed creating the test environment: ", err)
	}
	defer testEnv.Close(ctx)

	// p := &mempressure.RunParameters{
	// 	PageFilePath:             s.DataPath(mempressure.CompressibleData),
	// 	PageFileCompressionRatio: 0.40,
	// }

	// if err := mempressure.Run(ctx, s.OutDir(), testEnv.Chrome(), p); err != nil {
	// 	s.Fatal("Run failed: ", err)
	// }

	pgmem_cmd := testexec.CommandContext(ctx, "chromeos-pgmem")
	pgmem_out, pgmem_err := pgmem_cmd.Output()
	testing.ContextLogf(ctx, "%s %s", pgmem_out, pgmem_err)

	for i := 0; i < 6; i++ {
		// Give it time to settle
		sleepTime := 10 * time.Second
		if err := testing.Sleep(ctx, sleepTime); err != nil {
			s.Error("failed to sleep: ", err)
		}

		pgmem_cmd = testexec.CommandContext(ctx, "chromeos-pgmem")
		pgmem_out, pgmem_err = pgmem_cmd.Output()
		testing.ContextLogf(ctx, "%s %s", pgmem_out, pgmem_err)
	}

	// hack for i915 system
	gem_cmd := testexec.CommandContext(ctx, "cat", "/sys/kernel/debug/dri/0/i915_gem_objects")
	gem_out, gem_err := gem_cmd.Output()
	testing.ContextLogf(ctx, "GEM: %s %s", gem_out, gem_err)

	// hack for available -- do this with d-bus to resourced
	avail_cmd := testexec.CommandContext(ctx, "cat", "/sys/kernel/mm/chromeos-low_mem/available")
	avail_out, avail_err := avail_cmd.Output()
	testing.ContextLogf(ctx, "available: %s %s", avail_out, avail_err)
}
