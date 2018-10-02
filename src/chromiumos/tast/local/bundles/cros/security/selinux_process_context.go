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
				s.Errorf("Process %+v has context %v; want %v", proc, proc.SEContext, context)
			}
		}
	}

	ps, err := selinux.GetProcesses()
	if err != nil {
		s.Fatal("Failed to get processes: ", err)
	}

	type searchType int
	const (
		exe searchType = iota
	)
	for _, testCase := range []struct {
		query   string
		field   searchType
		context string
	}{
		{"/sbin/init", exe, "u:r:cros_init:s0"},
		{"/sbin/udevd", exe, "u:r:cros_udevd:s0"},
		{"/usr/bin/anomaly_collector", exe, "u:r:cros_anomaly_collector:s0"},
		{"/usr/sbin/rsyslogd", exe, "u:r:cros_rsyslogd:s0"},
	} {
		var p []selinux.Process
		switch testCase.field {
		case exe:
			p = selinux.FindProcessesByExe(ps, testCase.query)
			if len(p) == 0 {
				s.Errorf("Cannot find processes for exe %q", testCase.query)
			}
		default:
			s.Errorf("%+v has invalid searchType %d", testCase, int(testCase.field))
			continue
		}
		assertContext(p, testCase.context)
	}
}
