// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CheckProcesses,
		Desc:     "Checks that all expected processes are running",
		Contacts: []string{"nya@chromium.org"},
	})
}

func CheckProcesses(ctx context.Context, s *testing.State) {
	// Separate process names with | to allow multiple choices.
	expected := []string{
		"anomaly_detector",
		"conntrackd|netfilter-queue-helper",
		"dbus-daemon",
		"debugd",
		"metrics_daemon",
		"powerd",
		"shill",
		"systemd-udevd|udevd",
		"tlsdated",
		"update_engine",
		"wpa_supplicant",
	}

	// Insert names into a map for easy deletion later.
	needed := make(map[string]struct{}, len(expected))
	for _, n := range expected {
		needed[n] = struct{}{}
	}

	testing.Poll(ctx, func(ctx context.Context) error {
		procs, err := process.Processes()
		if err != nil {
			s.Fatal("Failed to get process listing: ", err)
		}
		running := make(map[string]int32)
		for _, proc := range procs {
			if name, err := proc.Name(); err == nil {
				running[name] = proc.Pid
			}
		}

		for names := range needed {
			for _, name := range strings.Split(names, "|") {
				if pid, ok := running[name]; ok {
					s.Logf("%v is running as %d", name, pid)
					delete(needed, names)
					break
				}
			}
		}
		if len(needed) > 0 {
			return errors.New("still waiting")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 30 * time.Second})

	// Iterate through expected rather than needed to report errors in a consistent order.
	for _, names := range expected {
		if _, ok := needed[names]; ok {
			s.Errorf("%v not running", names)
		}
	}
}
