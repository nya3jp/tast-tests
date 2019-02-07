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
		Contacts: []string{
			"kirtika@chromium.org",     // Connectivity team
			"briannorris@chromium.org", // Tast port author
		},
	})
}

func ShillStability(ctx context.Context, s *testing.State) {
	const (
		stabilityInterval = time.Second
		stabilityDuration = 20 * time.Second
	)

	// Returns PID of main shill process. Calls s.Fatal if we can't find
	// shill.
	getPID := func() int {
		const shillExecPath = "/usr/bin/shill"
		// Only look for shill as child of one of these.
		var shillParents = []string{"/sbin/minijail0", "/sbin/init"}

		all, err := process.Processes()
		if err != nil {
			s.Fatal("Failed to get process list: ", err)
		}

		for _, proc := range all {
			if exe, err := proc.Exe(); err != nil || exe != shillExecPath {
				continue
			}
			ppid, err := proc.Ppid()
			if err != nil {
				continue
			}
			parent, err := process.NewProcess(ppid)
			if err != nil {
				continue
			}
			if exe, err := parent.Exe(); err == nil {
				for _, p := range shillParents {
					if exe == p {
						return int(proc.Pid)
					}
				}
				continue
			}
		}
		s.Fatal("Could not find shill")
		panic("unreachable")
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
