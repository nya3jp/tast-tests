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
		Desc:         "Checks that processes are running in correct SELinux domain",
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxProcessContext(ctx context.Context, s *testing.State) {
	assertContext := func(processes []selinux.Process, context string) {
		for _, proc := range processes {
			if proc.SEContext != context {
				s.Errorf("Process %v has context %v; want %v", proc, proc.SEContext, context)
			}
		}
	}

	ps, err := selinux.GetProcesses()
	if err != nil {
		s.Fatal("Failed to get processes: ", err)
	}

	byExe := func(exe string, context string) {
		p := selinux.SearchProcessByExe(ps, exe)
		if len(p) == 0 {
			s.Errorf("Cannot find process where exe = %q", exe)
		}
		assertContext(p, context)
	}
	byExe("/sbin/init", "u:r:cros_init:s0")
	byExe("/sbin/udevd", "u:r:cros_udevd:s0")
	byExe("/usr/bin/anomaly_collector", "u:r:cros_anomaly_collector:s0")
	byExe("/usr/sbin/rsyslogd", "u:r:cros_rsyslogd:s0")
}
