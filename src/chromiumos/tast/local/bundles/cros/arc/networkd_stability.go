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
	// Returns the PIDs of both arc-networkd processes. This function
	// enforces the expectation that only two processes exist, which
	// can break if called at the same time the main service launches
	// or when the ARC container is booting up or tearing down since
	// external commands are invoked.
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
					s.Error("Failed to obtain parent PID for: ", proc.Pid)
				}
				if ppid == 1 {
					if mgr {
						s.Error("Found multiple manager processes")
					}
					mgr = true
				}
				pids = append(pids, int(proc.Pid))
			}
		}
		if !mgr {
			s.Error("Manager process not found")
		}
		if len(pids) != 2 {
			s.Errorf("Unexpected number of processes; got %d, wanted 2", len(pids))
		}
		sort.Ints(pids)
		return pids
	}

	checkPIDs := func(a, b []int) {
		if !reflect.DeepEqual(a, b) {
			s.Fatalf("PID changed: %v -> %v", a, b)
		}
		s.Logf("PIDs %v stable", a)
	}

	// Starts Chrome with ARC enabled, waits for Android to finish booting and then shuts it down.
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
	// at this point. This is important to note because when it first starts
	// it runs various external commands to setup the bridge interfaces.
	// As noted above, we want to make sure this has completed before retrieving
	// the PIDs.
	upstart.RestartJob(ctx, "ui")
	pids := getPIDs()

	// Ensure the processes are stable across ARC usage.
	// arc-networkd runs additional external commands when the ARC container is
	// starting and tearing down, so we need to wait for this complete before
	// checking the PIDs again (when doARC returns this will be true).
	doARC()
	checkPIDs(pids, getPIDs())

	// Ensure the processes are stable across logout.
	upstart.RestartJob(ctx, "ui")
	checkPIDs(pids, getPIDs())
}
