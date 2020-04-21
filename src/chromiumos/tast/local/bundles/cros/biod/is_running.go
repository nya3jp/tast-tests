// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package biod

import (
	"context"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: IsRunning,
		Desc: "Checks that biod is running on devices with fingerprint sensor",
		Contacts: []string{
			"yichengli@chromium.org", // Test author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
	})
}

// IsRunning checks the biod job and fails if it isn't running or has a process
// in the zombie state.
func IsRunning(ctx context.Context, s *testing.State) {
	if goal, state, pid, err := upstart.JobStatus(ctx, "biod"); err != nil {
		s.Fatal("Failed to get biod status")
	} else if goal != upstart.StartGoal || state != upstart.RunningState {
		s.Fatalf("Biod is not running (goal: %v / state: %v)", goal, state)
	} else if proc, err := process.NewProcess(int32(pid)); err != nil {
		s.Fatalf("Failed to check biod process %d", pid)
	} else if status, err := proc.Status(); err != nil {
		s.Fatalf("Failed to get biod process %d status", pid)
	} else if status == "Z" {
		s.Fatalf("Biod process %d is a zombie", pid)
	}
}
