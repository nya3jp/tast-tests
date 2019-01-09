// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"sort"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NetworkdStability,
		Desc:         "Checks that arc-networkd isn't respawning across ARC boots",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

func NetworkdStability(ctx context.Context, s *testing.State) {
	getPIDs := func() []int {
		const binPath = "/usr/bin/arc-networkd"

		all, err := process.Processes()
		if err != nil {
			s.Fatal("Failed to get process list: ", err)
		}

		var pids []int
		for _, proc := range all {
			if exe, _ := proc.Exe(); exe == binPath {
				pids = append(pids, int(proc.Pid))
			}
		}
		if len(pids) != 2 {
			s.Fatalf("Unexpected number of processes; got %d, wanted 2", len(pids))
		}
		sort.Ints(pids)
		return pids
	}

	checkPIDs := func(a, b []int) {
		for i := range b {
			if b[i] != a[i] {
				s.Fatalf("PID changed: %d -> %d", a[i], b[i])
			}
		}
		s.Logf("PIDs %v stable", a)
	}

	// Boots Chrome and ARC and returns the arc-networkd PIDs before shutdown.
	bootARC := func() []int {
		cr, err := chrome.New(ctx, chrome.ARCEnabled())
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}
		defer cr.Close(ctx)
		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		defer a.Close()
		return getPIDs()
	}

	pids := getPIDs()
	// Run through 2 ARC boot cycles and verify the PIDs are stable.
	checkPIDs(pids, bootARC())
	checkPIDs(pids, bootARC())
}
