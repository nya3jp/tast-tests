// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/mem"

	"chromiumos/tast/local/bundles/cros/hardware/memtester"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:    MemtesterSlow,
		Desc:    "Runs one iteration of memtester using 95% of free memory to find memory subsystem faults",
		Timeout: time.Hour,
		Contacts: []string{
			"puthik@chromium.org", // Original Autotest author
			"derat@chromium.org",  // Tast port author
			"cros-partner-avl@google.com",
		},
		Attr: []string{
			"disabled", // this test can be very slow
			"informational",
		},
	})
}

func MemtesterSlow(ctx context.Context, s *testing.State) {
	vmstat, err := mem.VirtualMemory()
	if err != nil {
		s.Fatal("Failed to get memory stats: ", err)
	}
	const mb = 1024 * 1024
	useBytes := int64(float64(vmstat.Free) * 0.95)

	s.Logf("Testing %.1f MiB (system is using %.1f of %.1f MB)",
		float64(useBytes)/mb, float64(vmstat.Used)/mb, float64(vmstat.Total)/mb)
	// TODO(derat): Switch this to perform 100 iterations and make it run during hardware
	// qualification (see the hardware_Memtester.memory_qual Autotest test). That can take
	// many hours to complete, so we should probably also parse the output from the memtester
	// process so we can log progress updates.
	if err := memtester.Run(ctx, useBytes, 1); err != nil {
		s.Fatal("memtester failed: ", err)
	}
}
