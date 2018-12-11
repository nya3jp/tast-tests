// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillStability,
		Desc: "Checks that shill isn't respawning",
	})
}

func ShillStability(ctx context.Context, s *testing.State) {
	const (
		stabilityInterval = time.Second
		stabilityDuration = 20 * time.Second
	)

	// Returns PID of main shill process. Calls s.Fatal if we can't find
	// shill, or there are too many non-parent/child instances.
	getPID := func() int {
		const shillExecPath = "/usr/bin/shill"

		all, err := process.Processes()
		if err != nil {
			s.Fatal("Failed to get process list: ", err)
		}

		var shillProc *process.Process
		for _, proc := range all {
			exe, err := proc.Exe()
			if err != nil || exe != shillExecPath {
				continue
			}
			ppid1, err := proc.Ppid()
			if err != nil {
				continue
			}
			if shillProc == nil {
				shillProc = proc
				continue
			}
			// We're a child of shill, so skip.
			if ppid1 == shillProc.Pid {
				continue
			}
			ppid2, err := shillProc.Ppid()
			// Other process is gone now, or we're the parent.
			if err != nil || ppid2 == proc.Pid {
				shillProc = proc
				continue
			}
			// Siblings: we'll find the common parent later.
			if ppid1 == ppid2 {
				continue
			}
			// Found 2 that weren't parent/child.
			s.Fatalf("Found 2 shill processes: %v, %v", shillProc.Pid, proc.Pid)
		}
		if shillProc == nil {
			s.Fatal("Could not find shill")
		}
		return int(shillProc.Pid)
	}

	pid := getPID()

	testing.Poll(ctx, func(ctx context.Context) error {
		if latest := getPID(); latest != pid {
			s.Fatalf("PID changed: %v -> %v", pid, latest)
		}
		s.Log("PID stable at: ", pid)
		return errors.New("keep polling")
	}, &testing.PollOptions{Timeout: stabilityDuration, Interval: stabilityInterval})
}
