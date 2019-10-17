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
		Contacts:     []string{"garrick@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_both", "chrome"},
		Timeout:      4 * time.Minute,
	})
}

func NetworkdStability(ctx context.Context, s *testing.State) {
	// Returns the PIDs of the arc-networkd processes. This function
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
			if exe, err := proc.Exe(); err != nil || exe != binPath {
				continue
			}
			ppid, err := proc.Ppid()
			if err != nil {
				s.Errorf("Failed to obtain parent PID for %v: %v", proc.Pid, err)
				continue
			}
			s.Logf("Found arc-networkd process %d with parent %d", proc.Pid, ppid)
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
		if len(pids) != 3 {
			s.Errorf("Unexpected number of processes; got %d, wanted 3", len(pids))
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
	// The arc-network-bridge job brings up arc-networkd but arc-network should not be running
	// if the container is down.
	if err := upstart.WaitForJobStatus(ctx, "arc-network-bridge", upstart.StartGoal, upstart.RunningState, upstart.RejectWrongGoal, 30*time.Second); err != nil {
		s.Fatal("arc-network-bridge job failed to start: ", err)
	}
	if err := upstart.WaitForJobStatus(ctx, "arc-network", upstart.StopGoal, upstart.WaitingState, upstart.RejectWrongGoal, 30*time.Second); err != nil {
		s.Fatal("arc-network job is unexpectedly running: ", err)
	}

	// Get the arc-networkd pids before logging in and starting ARC.
	pids := getPIDs()

	// Ensure the processes are stable across ARC usage.
	// arc-networkd runs additional external commands when the ARC container is
	// starting and tearing down, so we need to wait for this complete before
	// checking the PIDs again (when doARC returns this will be true).
	doARC()
	if err := upstart.WaitForJobStatus(ctx, "arc-network", upstart.StartGoal, upstart.RunningState, upstart.RejectWrongGoal, 30*time.Second); err != nil {
		s.Fatal("arc-network job failed to start: ", err)
	}
	checkPIDs(pids, getPIDs())

	// Ensure the processes are stable across logout.
	upstart.RestartJob(ctx, "ui")
	if err := upstart.WaitForJobStatus(ctx, "arc-network", upstart.StopGoal, upstart.WaitingState, upstart.RejectWrongGoal, 30*time.Second); err != nil {
		s.Fatal("arc-network job is unexpectedly running: ", err)
	}
	checkPIDs(pids, getPIDs())
}
