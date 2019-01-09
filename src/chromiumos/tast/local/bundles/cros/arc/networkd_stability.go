// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"reflect"
	"sort"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NetworkdStability,
		Desc:         "Checks that arc-networkd isn't respawning across ARC boots",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      6 * time.Minute,
	})
}

func NetworkdStability(ctx context.Context, s *testing.State) {
	getPIDs := func() []int {
		const binPath = "/usr/bin/arc-networkd"

		all, err := process.Processes()
		if err != nil {
			s.Fatal("Failed to get process list: ", err)
		}

		var (
			pids []int
			mgr  bool
		)
		for _, proc := range all {
			if exe, err := proc.Exe(); err == nil && exe == binPath {
				ppid, err := proc.Ppid()
				if err != nil {
					s.Errorf("Failed to obtain parent PID for: ", proc.Pid)
				}
				if ppid == 1 {
					if mgr {
						s.Errorf("Found multiple manager processes")
					}
					mgr = true
				}
				pids = append(pids, int(proc.Pid))
			}
		}
		if !mgr {
			s.Errorf("Manager process not found")
		}
		if len(pids) != 2 {
			s.Errorf("Unexpected number of processes; got %d, wanted 2", len(pids))
		}
		sort.Ints(pids)
		return pids
	}

	checkPIDs := func(a, b []int) {
		if !reflect.DeepEqual(a, b) {
			s.Fatal("PID changed: %v -> %v", a, b)
		}
		s.Logf("PIDs %v stable", a)
	}

	// Logs in Chrome with ARC enabled, waits for Android to finish booting and then shuts it down and logs out.
	doARC := func() {
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
	}

	// Ensure Chrome is logged out. arc-networkd must be up and quiescent
	// at this point.
	upstart.RestartJob(ctx, "ui")
	pids := getPIDs()

	// Ensure the processes are stable across ARC usage.
	doARC()
	checkPIDs(pids, getPIDs())

	// Ensure the processes are stable across logout.
	upstart.RestartJob(ctx, "ui")
	checkPIDs(pids, getPIDs())
}
