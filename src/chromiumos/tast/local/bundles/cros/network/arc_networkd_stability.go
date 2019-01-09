// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"sort"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcNetworkdStability,
		Desc:         "Checks that arc-networkd isn't respawning across ARC boots",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
	})
}

func ArcNetworkdStability(ctx context.Context, s *testing.State) {
	const (
		minBoots = 2
		maxBoots = 3
		timeout  = 5 * time.Minute
	)

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
		return pids
	}

	pids := getPIDs()
	sort.Ints(pids)

	var boots int
	testing.Poll(ctx, func(ctx context.Context) error {
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
		boots++

		latest := getPIDs()
		sort.Ints(latest)
		for i := range latest {
			if latest[i] != pids[i] {
				s.Fatalf("PID changed: %d -> %d", pids[i], latest[i])
			}
		}

		if boots < maxBoots {
			return errors.New("reboot")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})

	if boots < minBoots {
		s.Fatalf("Failed to boot ARC at least %d times", minBoots)
	}
	s.Logf("PIDs %v stable across %d reboots", pids, boots)
}
