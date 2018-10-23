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
		Attr: []string{"informational"},
	})
}

func ShillStability(ctx context.Context, s *testing.State) {
	const (
		stabilityInterval = time.Second
		stabilityDuration = 20 * time.Second
	)

	// GetPID returns PID of shill. Calls s.Fatal if we don't find exactly
	// 1 shill process.
	getPID := func() int {
		const shillExecPath = "/usr/bin/shill"

		all, err := process.Processes()
		if err != nil {
			s.Fatal("Failed to get process list: ", err)
		}

		var pids []int
		for _, proc := range all {
			if exe, err := proc.Exe(); err == nil && exe == shillExecPath {
				pids = append(pids, int(proc.Pid))
			}
		}
		if len(pids) != 1 {
			s.Fatalf("Found %v shill processes (%v); want 1", len(pids), pids)
		}
		return pids[0]
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
