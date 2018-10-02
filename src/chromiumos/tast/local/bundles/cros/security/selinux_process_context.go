// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/local/bundles/cros/security/selinux"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxProcessContext,
		Desc:         "Checks that processes are running at correct SELinux domain",
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxProcessContext(ctx context.Context, s *testing.State) {
	assertContext := func(processes []selinux.Process, context string) {
		for _, proc := range processes {
			if pc, err := selinux.GetSEContext(proc); pc != context {
				if err != nil {
					s.Errorf("Failed to get context for process %q: %q", proc, err)
				} else {
					s.Errorf("Process %q has context %v; want %v", proc, pc, context)
				}
			}
		}
	}

	ps, err := selinux.GetProcesses()
	if err != nil {
		s.Fatalf("Failed to get processes: %q", err)
	}

	byExe := func(exe string, context string) {
		p := selinux.SearchProcessByExe(ps, exe)
		if len(p) == 0 {
			s.Errorf("Cannot find process where exe = %v", exe)
		}
		assertContext(p, context)
	}
	byExe("/sbin/init", "u:r:cros_init:s0")
	byExe("/sbin/udevd", "u:r:cros_udevd:s0")
	byExe("/usr/bin/anomaly_collector", "u:r:cros_anomaly_collector:s0")
	byExe("/usr/sbin/rsyslogd", "u:r:cros_rsyslogd:s0")
}
