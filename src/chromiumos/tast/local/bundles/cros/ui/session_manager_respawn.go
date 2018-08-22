// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"fmt"

	"chromiumos/tast/local/bundles/cros/ui/respawn"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"

	"github.com/shirou/gopsutil/process"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SessionManagerRespawn,
		Desc:         "Checks that session_manager respawns after exit",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func SessionManagerRespawn(s *testing.State) {
	const sessionManagerPath = "/sbin/session_manager"
	getPID := func() (int, error) {
		all, err := process.Pids()
		if err != nil {
			return -1, err
		}

		for _, pid := range all {
			if proc, err := process.NewProcess(pid); err != nil {
				// Assume that the process exited.
				continue
			} else if exe, err := proc.Exe(); err == nil && exe == sessionManagerPath {
				return int(pid), nil
			}
		}
		return -1, fmt.Errorf("%v process not found", sessionManagerPath)
	}

	if err := upstart.EnsureJobRunning(s.Context(), "ui"); err != nil {
		s.Fatal("Failed to ensure ui job is running: ", err)
	}
	respawn.TestRespawn(s, "session_manager", getPID)
}
