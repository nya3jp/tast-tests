// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/testing"
)

// GetPID returns PID of shill. Raises an error if we don't find exactly 1
// shill process.
func getPID(s *testing.State) int {
	const (
		shillExecPath = "/usr/bin/shill"
	)

	all, err := process.Processes()
	if err != nil {
		s.Fatal(err)
	}

	pids := make([]int, 0)
	for _, proc := range all {
		if exe, err := proc.Exe(); err == nil && exe == shillExecPath {
			pids = append(pids, int(proc.Pid))
		}
	}
	if len(pids) != 1 {
		s.Fatal("Unexpected number of shill processes: ", pids)
	}
	return pids[0]
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillStability,
		Desc: "Checks that shill isn't respawning",
		Attr: []string{"informational"},
	})
}

func ShillStability(ctx context.Context, s *testing.State) {
	const (
		stabilityIntervalSeconds = 6
		stabilitySeconds         = 30
	)

	pid := getPID(s)

	ctx, cancel := context.WithTimeout(ctx, stabilitySeconds*time.Second)
	defer cancel()

	for true {
		select {
		case <-time.After(stabilityIntervalSeconds * time.Second):
			if latest := getPID(s); latest != pid {
				s.Fatalf("PID changed: %v -> %v", pid, latest)
			}
			s.Log("PID stable at: ", pid)
		case <-ctx.Done():
			// We reached the time limit without failures.
			return
		}
	}
}
