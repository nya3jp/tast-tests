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
		Desc:         "Checks that processes are running at correct SELinux domain.",
		Attr:         []string{},
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxProcessContext(ctx context.Context, s *testing.State) {
	assertContext := func(processes []selinux.Process, context string) {
		for _, proc := range processes {
			if selinux.GetSEContext(proc) != context+"\x00" {
				s.Errorf("Process %q has wrong SELinux domain, expect %v", proc, context)
			}
		}
	}
	byExe := func(exe string) []selinux.Process {
		p := selinux.SearchProcessByExe(exe)
		if len(p) == 0 {
			s.Errorf("Cannot find process where exe = %v", exe)
		}
		return p
	}
	assertContext(byExe("/sbin/init"), "u:r:cros_init:s0")
	assertContext(byExe("/sbin/udevd"), "u:r:cros_udevd:s0")
	assertContext(byExe("/usr/bin/anomaly_collector"), "u:r:cros_anomaly_collector:s0")
	assertContext(byExe("/usr/sbin/rsyslogd"), "u:r:cros_rsyslogd:s0")
}
