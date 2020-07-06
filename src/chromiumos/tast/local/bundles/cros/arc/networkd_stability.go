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
		Desc:         "Checks that platformd isn't respawning across ARC boots",
		Contacts:     []string{"garrick@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		// TODO(yusukes): Change the timeout back to 4 min when we revert arc.go's BootTimeout to 120s.
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func NetworkdStability(ctx context.Context, s *testing.State) {
	// Returns the PIDs of the patchpaneld processes.
	getPIDs := func() []int {
		const binPath = "/usr/bin/patchpaneld"

		all, err := process.Processes()
		if err != nil {
			s.Fatal("Failed to get process list: ", err)
		}

		var (
			pids []int
			mgr  bool
		)
		for _, proc := range all {
			if exe, err := proc.Exe(); err != nil || exe != binPath {
				continue
			}
			ppid, err := proc.Ppid()
			if err != nil {
				s.Errorf("Failed to obtain parent PID for %v: %v", proc.Pid, err)
				continue
			}
			s.Logf("Found patchpaneld process %d with parent %d", proc.Pid, ppid)
			if ppid == 1 {
				if mgr {
					s.Error("Found multiple manager processes")
				}
				mgr = true
			}
			pids = append(pids, int(proc.Pid))
		}
		if !mgr {
			s.Error("Manager process not found")
		}
		sort.Ints(pids)
		return pids
	}

	checkPIDs := func(a, b []int) {
		if !reflect.DeepEqual(a, b) {
			s.Fatalf("PIDs changed: %v -> %v", a, b)
		}
		s.Logf("PIDs %v stable", a)
	}

	// Starts Chrome with ARC enabled, waits for Android to finish booting.
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

	// Log out to ensure the container is down.
	upstart.RestartJob(ctx, "ui")

	// Ensure the daemon is up and running and in a known state.
	if err := upstart.WaitForJobStatus(ctx, "patchpanel", upstart.StartGoal, upstart.RunningState, upstart.RejectWrongGoal, 30*time.Second); err != nil {
		s.Fatal("patchpanel job failed to start: ", err)
	}

	// Get thepatchpaneld pids before logging in and starting ARC.
	pids := getPIDs()

	// Ensure the processes are stable across ARC usage.
	// patchpaneld runs additional external commands when the ARC container is
	// starting and tearing down, so we need to wait for this complete before
	// checking the PIDs again (when doARC returns this will be true).
	doARC()
	checkPIDs(pids, getPIDs())

	// Ensure the processes are stable across logout.
	upstart.RestartJob(ctx, "ui")
	checkPIDs(pids, getPIDs())
}
